// Unified media player wrapping either the YouTube IFrame API or an
// HTMLAudioElement behind a single interface. The mode is picked from the
// source URL: YouTube links use the iframe player, everything else falls back
// to an <audio> tag (with an optional thumbnail).
//
// State values mirror the YT.PlayerState enum so callers can compare against
// MediaPlayerState.PLAYING regardless of backend.
(function () {
    const PLAYER_STATE = {
        UNSTARTED: -1,
        ENDED: 0,
        PLAYING: 1,
        PAUSED: 2,
        BUFFERING: 3,
        CUED: 5,
    };

    function isYouTubeURL(url) {
        if (!url) return false;
        const u = String(url).toLowerCase();
        return u.includes('youtube.com') || u.includes('youtu.be');
    }

    function extractYouTubeId(url) {
        if (!url) return null;
        const patterns = [
            /(?:youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/embed\/|youtube\.com\/v\/)([a-zA-Z0-9_-]{11})/,
            /youtube\.com\/watch\?.*v=([a-zA-Z0-9_-]{11})/,
        ];
        for (const p of patterns) {
            const m = String(url).match(p);
            if (m && m[1]) return m[1];
        }
        return null;
    }

    const VOLUME_STORAGE_KEY = 'syncrate-volume';
    function loadPersistedVolume() {
        try {
            const raw = window.localStorage && window.localStorage.getItem(VOLUME_STORAGE_KEY);
            const n = parseFloat(raw);
            if (!isNaN(n) && n >= 0 && n <= 150) return n;
        } catch (e) {}
        return 100;
    }
    function savePersistedVolume(pct) {
        try {
            if (window.localStorage) window.localStorage.setItem(VOLUME_STORAGE_KEY, String(pct));
        } catch (e) {}
    }

    let ytApiPromise = null;
    function ensureYouTubeAPI() {
        if (window.YT && window.YT.Player) return Promise.resolve();
        if (ytApiPromise) return ytApiPromise;
        ytApiPromise = new Promise((resolve) => {
            const prev = window.onYouTubeIframeAPIReady;
            window.onYouTubeIframeAPIReady = function () {
                if (typeof prev === 'function') { try { prev(); } catch (e) {} }
                resolve();
            };
            if (!document.querySelector('script[src*="youtube.com/iframe_api"]')) {
                const tag = document.createElement('script');
                tag.src = 'https://www.youtube.com/iframe_api';
                document.head.appendChild(tag);
            } else if (window.YT && window.YT.Player) {
                // Script already loaded between the initial check and now.
                resolve();
            }
        });
        return ytApiPromise;
    }

    class MediaPlayer {
        constructor(opts) {
            opts = opts || {};
            this.containerId = opts.containerId;
            this.sourceURL = opts.sourceURL || '';
            this.thumbnailURL = opts.thumbnailURL || '';
            this.width = opts.width;
            this.height = opts.height;
            this.controls = opts.controls !== false;
            this.autoplay = !!opts.autoplay;
            this.showThumbnail = opts.showThumbnail !== false;
            this.playerVars = opts.playerVars || {};
            this.onReady = opts.onReady;
            this.onStateChange = opts.onStateChange;
            this.onEnded = opts.onEnded;
            this.onError = opts.onError;
            this.ready = false;
            this.destroyed = false;
            this._pendingCue = null;
            // Volume in 0-150% range. YT clamps to 100; native audio uses a
            // Web Audio GainNode to boost above 100. Defaults to whatever is
            // persisted in localStorage so every room starts at the user's
            // preferred level. Pass opts.volume to override.
            this._volumePct = typeof opts.volume === 'number'
                ? Math.max(0, Math.min(150, opts.volume))
                : loadPersistedVolume();
            this._audioCtx = null;
            this._gainNode = null;
            this._audioSource = null;
            this.isYouTube = isYouTubeURL(this.sourceURL);
            if (this.sourceURL) {
                this._build();
            }
        }

        _container() {
            return document.getElementById(this.containerId);
        }

        _build() {
            const container = this._container();
            if (!container) return;
            container.innerHTML = '';
            if (this.isYouTube) {
                this._buildYouTube(container);
            } else {
                this._buildAudio(container);
            }
        }

        _buildYouTube(container) {
            const inner = document.createElement('div');
            container.appendChild(inner);
            const videoId = extractYouTubeId(this.sourceURL);
            ensureYouTubeAPI().then(() => {
                if (this.destroyed) return;
                const playerVars = Object.assign({
                    autoplay: this.autoplay ? 1 : 0,
                    controls: this.controls ? 1 : 0,
                    modestbranding: 1,
                    rel: 0,
                }, this.playerVars);
                const opts = {
                    playerVars,
                    events: {
                        onReady: () => {
                            this.ready = true;
                            this._applyVolume();
                            // YouTube exposes no volume-change event, so poll
                            // its native control to persist user adjustments.
                            if (this.controls) this._startYouTubeVolumePolling();
                            if (this.onReady) this.onReady();
                            if (this._pendingCue) {
                                const c = this._pendingCue;
                                this._pendingCue = null;
                                this.cue(c);
                            }
                        },
                        onStateChange: (event) => {
                            if (this.onStateChange) this.onStateChange(event.data);
                            if (event.data === window.YT.PlayerState.ENDED && this.onEnded) this.onEnded();
                        },
                        onError: (event) => {
                            if (this.onError) this.onError(event.data);
                        },
                    },
                };
                if (this.width !== undefined) opts.width = this.width;
                if (this.height !== undefined) opts.height = this.height;
                // Skip the initial videoId when a pending cue is queued — the
                // pending cue will load the right video (potentially with a
                // startSeconds offset) without us double-loading here.
                if (videoId && !this._pendingCue) opts.videoId = videoId;
                this._yt = new window.YT.Player(inner, opts);
            });
        }

        _buildAudio(container) {
            const wrap = document.createElement('div');
            wrap.className = 'media-audio-wrap';

            if (this.showThumbnail && this.thumbnailURL) {
                const img = document.createElement('img');
                img.src = this.thumbnailURL;
                img.className = 'media-audio-thumbnail';
                img.alt = '';
                wrap.appendChild(img);
            }

            const audio = document.createElement('audio');
            audio.src = this.sourceURL;
            audio.preload = 'metadata';
            audio.className = 'media-audio-element';
            if (this.controls) audio.controls = true;
            if (this.autoplay) audio.autoplay = true;

            audio.addEventListener('loadedmetadata', () => {
                if (this.destroyed) return;
                if (!this.ready) {
                    this.ready = true;
                    this._applyVolume();
                    if (this.onReady) this.onReady();
                    if (this.onStateChange) this.onStateChange(PLAYER_STATE.CUED);
                    if (this._pendingCue) {
                        const c = this._pendingCue;
                        this._pendingCue = null;
                        this.cue(c);
                    }
                }
            });
            audio.addEventListener('play', () => {
                if (this.onStateChange) this.onStateChange(PLAYER_STATE.PLAYING);
            });
            audio.addEventListener('pause', () => {
                if (audio.ended) return; // 'ended' fires its own event below.
                if (this.onStateChange) this.onStateChange(PLAYER_STATE.PAUSED);
            });
            audio.addEventListener('ended', () => {
                if (this.onStateChange) this.onStateChange(PLAYER_STATE.ENDED);
                if (this.onEnded) this.onEnded();
            });
            audio.addEventListener('waiting', () => {
                if (this.onStateChange) this.onStateChange(PLAYER_STATE.BUFFERING);
            });
            audio.addEventListener('error', () => {
                if (this.onError) this.onError(audio.error ? audio.error.code : 0);
            });
            // Persist user-driven volume changes from the native controls.
            // Programmatic writes in _applyVolume tag _appliedAudioVolume so we
            // can tell them apart and ignore them here. Native controls top out
            // at 100%, so a user change collapses any boost graph and treats
            // audio.volume as the source of truth.
            audio.addEventListener('volumechange', () => {
                if (this.destroyed) return;
                if (this._appliedAudioVolume != null &&
                    Math.abs(audio.volume - this._appliedAudioVolume) < 1e-3) return;
                const pct = Math.round(audio.volume * 100);
                this._volumePct = pct;
                if (this._gainNode) {
                    try { this._gainNode.gain.value = 1; } catch (e) {}
                }
                savePersistedVolume(pct);
            });

            wrap.appendChild(audio);
            container.appendChild(wrap);
            this._audio = audio;
        }

        isYouTubeMode() { return this.isYouTube; }
        isReady() { return this.ready; }

        // Load (and pause at) a track at startSeconds. If sourceURL flips
        // between YouTube and direct media, the underlying backend is
        // recreated.
        cue(opts) {
            opts = opts || {};
            const newURL = opts.sourceURL;
            const startSeconds = opts.startSeconds;

            // First-time build (constructor was called without a source).
            if (newURL && !this._yt && !this._audio) {
                this.sourceURL = newURL;
                this.isYouTube = isYouTubeURL(newURL);
                this.ready = false;
                this._pendingCue = { startSeconds };
                this._build();
                return;
            }

            if (newURL && newURL !== this.sourceURL) {
                const newIsYT = isYouTubeURL(newURL);
                if (newIsYT !== this.isYouTube) {
                    // Backend change required — tear down and rebuild.
                    this._teardown();
                    this.sourceURL = newURL;
                    this.isYouTube = newIsYT;
                    this.ready = false;
                    this._pendingCue = { startSeconds };
                    this._build();
                    return;
                }
                this.sourceURL = newURL;
            }

            if (!this.ready) {
                this._pendingCue = { sourceURL: newURL, startSeconds };
                return;
            }

            if (this.isYouTube) {
                if (!this._yt) return;
                const vidId = extractYouTubeId(this.sourceURL);
                if (!vidId) return;
                const cueOpts = { videoId: vidId };
                if (typeof startSeconds === 'number') cueOpts.startSeconds = startSeconds;
                try { this._yt.cueVideoById(cueOpts); } catch (e) {}
            } else {
                if (!this._audio) return;
                if (newURL) {
                    this._audio.src = newURL;
                    this._audio.load();
                }
                const setStart = () => {
                    if (typeof startSeconds === 'number') {
                        try { this._audio.currentTime = startSeconds; } catch (e) {}
                    }
                };
                if (this._audio.readyState >= 1) setStart();
                else this._audio.addEventListener('loadedmetadata', setStart, { once: true });
                try { this._audio.pause(); } catch (e) {}
            }
        }

        play() {
            if (this.isYouTube) {
                if (this._yt && this.ready) {
                    try { this._yt.playVideo(); } catch (e) {}
                }
            } else if (this._audio) {
                // Browsers auto-suspend the AudioContext until a gesture.
                // Calling play() is itself a gesture, so resume here.
                if (this._audioCtx && this._audioCtx.state === 'suspended') {
                    try { this._audioCtx.resume(); } catch (e) {}
                }
                const p = this._audio.play();
                if (p && typeof p.catch === 'function') p.catch(() => {});
            }
        }

        // pct is 0-150. Cached so it can be applied once the player becomes
        // ready, or re-applied after a backend switch destroys the audio
        // element.
        setVolume(pct) {
            const v = Math.max(0, Math.min(150, Number(pct) || 0));
            this._volumePct = v;
            if (!this.ready) return;
            this._applyVolume();
        }

        getVolume() {
            return this._volumePct != null ? this._volumePct : 100;
        }

        _applyVolume() {
            if (this._volumePct == null) return;
            const pct = this._volumePct;
            if (this.isYouTube) {
                if (this._yt) {
                    try { this._yt.setVolume(Math.min(100, pct)); } catch (e) {}
                }
                return;
            }
            if (!this._audio) return;
            const ratio = pct / 100;
            // Record the value we're about to write so the native
            // 'volumechange' listener can distinguish our own writes from the
            // user dragging the control.
            if (ratio > 1) {
                this._ensureGainGraph();
                if (this._gainNode) {
                    this._gainNode.gain.value = ratio;
                    this._appliedAudioVolume = 1;
                    this._audio.volume = 1;
                } else {
                    // No Web Audio support: silently clamp at 100%.
                    this._appliedAudioVolume = 1;
                    this._audio.volume = 1;
                }
            } else if (this._gainNode) {
                // Graph already wired up from an earlier boost; keep using it.
                this._gainNode.gain.value = ratio;
                this._appliedAudioVolume = 1;
                this._audio.volume = 1;
            } else {
                this._appliedAudioVolume = ratio;
                this._audio.volume = ratio;
            }
        }

        _startYouTubeVolumePolling() {
            this._stopYouTubeVolumePolling();
            this._ytVolPoll = setInterval(() => {
                if (this.destroyed || !this._yt || !this.ready) return;
                let v;
                try { v = this._yt.getVolume(); } catch (e) { return; }
                if (typeof v !== 'number') return;
                // YT applies min(100, pct); compare against that so re-applying
                // a boosted 150% from another room doesn't read back as a change.
                const applied = Math.min(100, this._volumePct == null ? 100 : this._volumePct);
                if (Math.abs(v - applied) >= 1) {
                    this._volumePct = v;
                    savePersistedVolume(v);
                }
            }, 1000);
        }

        _stopYouTubeVolumePolling() {
            if (this._ytVolPoll) {
                clearInterval(this._ytVolPoll);
                this._ytVolPoll = null;
            }
        }

        _ensureGainGraph() {
            if (this._gainNode || !this._audio) return;
            const AC = window.AudioContext || window.webkitAudioContext;
            if (!AC) return;
            try {
                if (!this._audioCtx) this._audioCtx = new AC();
                // createMediaElementSource can only be called once per
                // element; we rebuild on backend switch via _teardown.
                const source = this._audioCtx.createMediaElementSource(this._audio);
                const gain = this._audioCtx.createGain();
                source.connect(gain);
                gain.connect(this._audioCtx.destination);
                this._audioSource = source;
                this._gainNode = gain;
            } catch (e) {
                this._audioSource = null;
                this._gainNode = null;
            }
        }

        pause() {
            if (this.isYouTube) {
                if (this._yt && this.ready) {
                    try { this._yt.pauseVideo(); } catch (e) {}
                }
            } else if (this._audio) {
                try { this._audio.pause(); } catch (e) {}
            }
        }

        seekTo(seconds, allowSeekAhead) {
            if (allowSeekAhead === undefined) allowSeekAhead = true;
            if (this.isYouTube) {
                if (this._yt && this.ready) {
                    try { this._yt.seekTo(seconds, allowSeekAhead); } catch (e) {}
                }
            } else if (this._audio) {
                try { this._audio.currentTime = seconds; } catch (e) {}
            }
        }

        getCurrentTime() {
            if (this.isYouTube) {
                if (!this._yt || !this.ready) return 0;
                try { return this._yt.getCurrentTime() || 0; } catch (e) { return 0; }
            }
            return this._audio ? (this._audio.currentTime || 0) : 0;
        }

        getDuration() {
            if (this.isYouTube) {
                if (!this._yt || !this.ready) return 0;
                try { return this._yt.getDuration() || 0; } catch (e) { return 0; }
            }
            if (!this._audio) return 0;
            return isFinite(this._audio.duration) ? this._audio.duration : 0;
        }

        getState() {
            if (this.isYouTube) {
                if (!this._yt || !this.ready) return PLAYER_STATE.UNSTARTED;
                try { return this._yt.getPlayerState(); } catch (e) { return PLAYER_STATE.UNSTARTED; }
            }
            if (!this._audio) return PLAYER_STATE.UNSTARTED;
            if (this._audio.ended) return PLAYER_STATE.ENDED;
            if (this._audio.paused) return PLAYER_STATE.PAUSED;
            return PLAYER_STATE.PLAYING;
        }

        _teardown() {
            this._stopYouTubeVolumePolling();
            if (this._yt) {
                try { this._yt.destroy(); } catch (e) {}
                this._yt = null;
            }
            if (this._audio) {
                try { this._audio.pause(); } catch (e) {}
                this._audio.src = '';
                this._audio = null;
            }
            // The Web Audio source is bound to the audio element we just
            // dropped — release it so the next backend can build a fresh
            // graph. The AudioContext itself is kept for reuse.
            if (this._audioSource) {
                try { this._audioSource.disconnect(); } catch (e) {}
                this._audioSource = null;
            }
            if (this._gainNode) {
                try { this._gainNode.disconnect(); } catch (e) {}
                this._gainNode = null;
            }
            const container = this._container();
            if (container) container.innerHTML = '';
            this.ready = false;
        }

        destroy() {
            this.destroyed = true;
            this._teardown();
            if (this._audioCtx) {
                try { this._audioCtx.close(); } catch (e) {}
                this._audioCtx = null;
            }
        }
    }

    window.MediaPlayer = MediaPlayer;
    window.MediaPlayerState = PLAYER_STATE;
    window.isYouTubeURL = isYouTubeURL;
    window.loadPersistedVolume = loadPersistedVolume;
    window.savePersistedVolume = savePersistedVolume;
})();
