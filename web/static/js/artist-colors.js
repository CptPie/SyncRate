/**
 * Artist and Unit color styling functionality
 * Applies artist and unit colors to elements with readable text colors
 */

/**
 * Calculate the luminance of a color to determine if it's light or dark
 * @param {string} color - Hex color code (e.g., "#FF5733")
 * @returns {number} - Luminance value between 0 and 1
 */
function calculateLuminance(color) {
  // Remove # if present
  const hex = color.replace("#", "");

  // Parse RGB values
  const r = parseInt(hex.substr(0, 2), 16) / 255;
  const g = parseInt(hex.substr(2, 2), 16) / 255;
  const b = parseInt(hex.substr(4, 2), 16) / 255;

  // Apply gamma correction
  const rLinear = r <= 0.03928 ? r / 12.92 : Math.pow((r + 0.055) / 1.055, 2.4);
  const gLinear = g <= 0.03928 ? g / 12.92 : Math.pow((g + 0.055) / 1.055, 2.4);
  const bLinear = b <= 0.03928 ? b / 12.92 : Math.pow((b + 0.055) / 1.055, 2.4);

  // Calculate luminance
  return 0.2126 * rLinear + 0.7152 * gLinear + 0.0722 * bLinear;
}

/**
 * Determine if a color should use light or dark text for readability
 * @param {string} backgroundColor - Hex color code
 * @returns {string} - Either '#ffffff' (white) or '#000000' (black)
 */
function getReadableTextColor(backgroundColor) {
  if (!backgroundColor || backgroundColor === "") {
    return "#000000"; // Default to black text
  }

  const luminance = calculateLuminance(backgroundColor);
  // Use white text on dark backgrounds, black text on light backgrounds
  return luminance > 0.5 ? "#000000" : "#ffffff";
}

/**
 * Apply color styling to an element (works for both artists and units)
 * @param {HTMLElement} element - The element to style
 * @param {string} primaryColor - Primary color (can be null/empty)
 * @param {string} secondaryColor - Secondary color (optional)
 */
function applyElementColor(element, primaryColor, secondaryColor = null) {
  if (!element) {
    return;
  }

  // Use fallback CSS variable if no primary color is provided
  if (primaryColor) {
    // Set custom background color
    element.style.backgroundColor = primaryColor;

    // Set readable text color
    const textColor = getReadableTextColor(primaryColor);
    element.style.color = textColor;
  } else {
    // Use CSS variable for fallback - this will update automatically with theme changes
    element.style.backgroundColor = "var(--bg-accent)";
    element.style.color = "var(--text-primary)";
  }

  // Add some padding and border radius for better visual appearance
  element.style.padding = "4px 8px";
  element.style.borderRadius = "15px";
  element.style.display = "inline-block";
  element.style.margin = "2px";

  // Optional: Add secondary color as border if provided
  if (secondaryColor) {
    element.style.border = `2px solid ${secondaryColor}`;
  }
}

/**
 * Apply artist colors to all artist elements in song tiles
 * @param {Array} songData - Array of song objects with artist data
 */
function initializeArtistColors(songData) {
  if (!songData || !Array.isArray(songData)) {
    return;
  }

  songData.forEach((song) => {
    if (!song.Artists || !Array.isArray(song.Artists)) {
      return;
    }

    song.Artists.forEach((artist, index) => {
      // Find all artist elements for this song
      const songElements = document.querySelectorAll(
        `[data-song-id="${song.SongID}"]`,
      );
      songElements.forEach((songElement) => {
        const artistElements = songElement.querySelectorAll(".artist-name");
        if (artistElements[index]) {
          // Apply color styling - will use fallback if no PrimaryColor
          applyElementColor(
            artistElements[index],
            artist.PrimaryColor,
            artist.SecondaryColor,
          );
        }
      });
    });
  });
}

/**
 * Apply unit colors to all unit elements in song tiles
 * @param {Array} songData - Array of song objects with unit data
 */
function initializeUnitColors(songData) {
  if (!songData || !Array.isArray(songData)) {
    return;
  }

  songData.forEach((song) => {
    if (!song.Units || !Array.isArray(song.Units)) {
      return;
    }

    song.Units.forEach((unit, index) => {
      // Find all unit elements for this song
      const songElements = document.querySelectorAll(
        `[data-song-id="${song.SongID}"]`,
      );
      songElements.forEach((songElement) => {
        const unitElements = songElement.querySelectorAll(".unit-name");
        if (unitElements[index]) {
          // Apply color styling - will use fallback if no PrimaryColor
          applyElementColor(
            unitElements[index],
            unit.PrimaryColor,
            unit.SecondaryColor,
          );
        }
      });
    });
  });
}

/**
 * Initialize both artist and unit colors for song tiles
 * @param {Array} songData - Array of song objects with artist and unit data
 */
function initializeAllColors(songData) {
  initializeArtistColors(songData);
  initializeUnitColors(songData);
}
