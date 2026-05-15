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
                const p = this._audio.play();
                if (p && typeof p.catch === 'function') p.catch(() => {});
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
            if (this._yt) {
                try { this._yt.destroy(); } catch (e) {}
                this._yt = null;
            }
            if (this._audio) {
                try { this._audio.pause(); } catch (e) {}
                this._audio.src = '';
                this._audio = null;
            }
            const container = this._container();
            if (container) container.innerHTML = '';
            this.ready = false;
        }

        destroy() {
            this.destroyed = true;
            this._teardown();
        }
    }

    window.MediaPlayer = MediaPlayer;
    window.MediaPlayerState = PLAYER_STATE;
    window.isYouTubeURL = isYouTubeURL;
})();
