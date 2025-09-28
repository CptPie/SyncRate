/**
 * Color picker synchronization functionality
 * Synchronizes color picker input with hex text field
 */

/**
 * Setup color synchronization between a color picker and text input
 * @param {string} pickerId - ID of the color picker input element
 * @param {string} textId - ID of the text input element
 */
function setupColorSync(pickerId, textId) {
  const picker = document.getElementById(pickerId);
  const textInput = document.getElementById(textId);

  if (!picker || !textInput) {
    console.warn(`Color sync setup failed: Could not find elements with IDs '${pickerId}' and '${textId}'`);
    return;
  }

  // Update text field when color picker changes
  picker.addEventListener('input', function() {
    textInput.value = this.value.toUpperCase();
  });

  // Update color picker when text field changes
  textInput.addEventListener('input', function() {
    let value = this.value.trim().toUpperCase();

    // Auto-prepend # if missing but hex code is valid
    if (/^[0-9A-Fa-f]{6}$/.test(value)) {
      value = '#' + value;
      this.value = value;
    }

    if (/^#[0-9A-Fa-f]{6}$/.test(value)) {
      picker.value = value;
      // Remove invalid styling to let CSS handle theming
      this.style.borderColor = '';
      this.style.backgroundColor = '';
    } else if (value === '') {
      // Remove invalid styling to let CSS handle theming
      this.style.borderColor = '';
      this.style.backgroundColor = '';
    }
  });

  // Format hex value on blur
  textInput.addEventListener('blur', function() {
    let value = this.value.trim().toUpperCase();
    if (value && !value.startsWith('#')) {
      value = '#' + value;
    }
    if (/^#[0-9A-Fa-f]{6}$/.test(value)) {
      this.value = value;
      picker.value = value;
    } else if (value && value !== '') {
      // Invalid format - clear the field
      this.value = '';
    }
  });
}

/**
 * Initialize color synchronization for multiple color input pairs
 * @param {Array<{pickerId: string, textId: string}>} colorPairs - Array of picker/text ID pairs
 */
function initializeColorSync(colorPairs) {
  colorPairs.forEach(pair => {
    setupColorSync(pair.pickerId, pair.textId);
  });
}