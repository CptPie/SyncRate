// Theme Toggle Functionality
(function() {
    'use strict';

    // Get the current theme from localStorage or default to 'light'
    function getCurrentTheme() {
        return localStorage.getItem('theme') || 'light';
    }

    // Set the theme on the document
    function setTheme(theme) {
        if (theme === 'dark') {
            document.documentElement.setAttribute('data-theme', 'dark');
        } else {
            document.documentElement.removeAttribute('data-theme');
        }
        localStorage.setItem('theme', theme);
    }

    // Toggle between light and dark themes
    function toggleTheme() {
        const currentTheme = getCurrentTheme();
        const newTheme = currentTheme === 'light' ? 'dark' : 'light';
        setTheme(newTheme);
    }

    // Set up event listeners
    function setupEventListeners() {
        const themeToggleButton = document.getElementById('theme-toggle');
        if (themeToggleButton) {
            themeToggleButton.addEventListener('click', toggleTheme);
        }
    }

    // Initialize everything when DOM is loaded
    function init() {
        setupEventListeners();
    }

    // Run initialization
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

})();