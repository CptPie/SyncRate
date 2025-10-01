package utils

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// YouTubeVideoID extracts the video ID from various YouTube URL formats
func YouTubeVideoID(url string) (string, error) {
	// Common YouTube URL patterns
	patterns := []string{
		`(?:youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/embed\/|youtube\.com\/v\/)([a-zA-Z0-9_-]{11})`,
		`youtube\.com\/watch\?.*v=([a-zA-Z0-9_-]{11})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("could not extract video ID from URL: %s", url)
}

// IsYouTubeURL checks if the provided URL is a YouTube URL
func IsYouTubeURL(url string) bool {
	lowerURL := strings.ToLower(url)
	return strings.Contains(lowerURL, "youtube.com") || strings.Contains(lowerURL, "youtu.be")
}

// GetYouTubeThumbnailURL generates the highest resolution YouTube thumbnail URL available
func GetYouTubeThumbnailURL(videoID string) (string, error) {
	// YouTube thumbnail resolutions in order of preference (highest to lowest)
	resolutions := []string{
		"maxresdefault",  // 1280x720
		"sddefault",      // 640x480
		"hqdefault",      // 480x360
		"mqdefault",      // 320x180
		"default",        // 120x90
	}

	for _, resolution := range resolutions {
		thumbnailURL := fmt.Sprintf("https://img.youtube.com/vi/%s/%s.jpg", videoID, resolution)

		// Check if thumbnail exists by making a HEAD request
		resp, err := http.Head(thumbnailURL)
		if err != nil {
			continue
		}
		resp.Body.Close()

		// YouTube returns 404 for non-existent thumbnails
		if resp.StatusCode == http.StatusOK {
			return thumbnailURL, nil
		}
	}

	// Fallback to default thumbnail if all else fails
	return fmt.Sprintf("https://img.youtube.com/vi/%s/default.jpg", videoID), nil
}

// ExtractYouTubeThumbnail is a convenience function that combines URL detection,
// video ID extraction, and thumbnail URL generation
func ExtractYouTubeThumbnail(sourceURL string) (string, error) {
	if !IsYouTubeURL(sourceURL) {
		return "", fmt.Errorf("not a YouTube URL")
	}

	videoID, err := YouTubeVideoID(sourceURL)
	if err != nil {
		return "", err
	}

	return GetYouTubeThumbnailURL(videoID)
}

// GetYouTubeEmbedURL converts a YouTube URL to its embed format
func GetYouTubeEmbedURL(sourceURL string) (string, error) {
	if !IsYouTubeURL(sourceURL) {
		return "", fmt.Errorf("not a YouTube URL")
	}

	videoID, err := YouTubeVideoID(sourceURL)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://www.youtube.com/embed/%s", videoID), nil
}