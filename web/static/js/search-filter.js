// Search and Filter functionality for view pages
class SearchFilter {
    constructor(options) {
        this.searchInputId = options.searchInputId;
        this.categoryFilterId = options.categoryFilterId;
        this.itemsContainerId = options.itemsContainerId;
        this.itemClass = options.itemClass;
        this.noResultsId = options.noResultsId;
        this.searchFields = options.searchFields || []; // Array of field paths to search in
        this.data = options.data || [];
        this.categoriesData = options.categoriesData || [];

        this.filteredData = [...this.data];
        this.currentSearchTerm = '';
        this.currentCategoryFilter = '';

        this.init();
    }

    init() {
        this.setupEventListeners();
        this.populateCategoryFilter();
        this.renderItems();
    }

    setupEventListeners() {
        const searchInput = document.getElementById(this.searchInputId);
        const categoryFilter = document.getElementById(this.categoryFilterId);

        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                this.currentSearchTerm = e.target.value.toLowerCase().trim();
                this.filterAndRender();
            });
        }

        if (categoryFilter) {
            categoryFilter.addEventListener('change', (e) => {
                this.currentCategoryFilter = e.target.value;
                this.filterAndRender();
            });
        }
    }

    populateCategoryFilter() {
        const categoryFilter = document.getElementById(this.categoryFilterId);
        if (!categoryFilter || !this.categoriesData.length) return;

        // Clear existing options except "All Categories"
        categoryFilter.innerHTML = '<option value="">All Categories</option>';

        // Add category options
        this.categoriesData.forEach(category => {
            const option = document.createElement('option');
            option.value = category.CategoryID;
            option.textContent = category.Name;
            categoryFilter.appendChild(option);
        });
    }

    filterAndRender() {
        this.filteredData = this.data.filter(item => {
            return this.matchesSearch(item) && this.matchesCategory(item);
        });

        this.renderItems();
    }

    matchesSearch(item) {
        if (!this.currentSearchTerm) return true;

        return this.searchFields.some(fieldPath => {
            const value = this.getNestedValue(item, fieldPath);
            if (!value) return false;

            if (Array.isArray(value)) {
                // For arrays (like Artists, Units), search in each item
                return value.some(arrayItem => {
                    if (typeof arrayItem === 'string') {
                        return arrayItem.toLowerCase().includes(this.currentSearchTerm);
                    } else if (arrayItem && typeof arrayItem === 'object') {
                        // Search in NameOriginal and NameEnglish of array items
                        return (arrayItem.NameOriginal && arrayItem.NameOriginal.toLowerCase().includes(this.currentSearchTerm)) ||
                               (arrayItem.NameEnglish && arrayItem.NameEnglish.toLowerCase().includes(this.currentSearchTerm));
                    }
                    return false;
                });
            } else {
                return value.toLowerCase().includes(this.currentSearchTerm);
            }
        });
    }

    matchesCategory(item) {
        if (!this.currentCategoryFilter) return true;

        // Check if item has a category and it matches the filter
        const categoryId = item.Category?.CategoryID || item.CategoryID;
        return categoryId && categoryId.toString() === this.currentCategoryFilter;
    }

    getNestedValue(obj, path) {
        return path.split('.').reduce((current, key) => {
            return current && current[key] !== undefined ? current[key] : null;
        }, obj);
    }

    renderItems() {
        const container = document.getElementById(this.itemsContainerId);
        const noResults = document.getElementById(this.noResultsId);

        if (!container) return;

        // Hide all existing items
        const allItems = container.querySelectorAll('.' + this.itemClass);
        allItems.forEach(item => {
            item.style.display = 'none';
        });

        if (this.filteredData.length === 0) {
            if (noResults) {
                noResults.style.display = 'block';
            }
            return;
        }

        if (noResults) {
            noResults.style.display = 'none';
        }

        // Show matching items
        this.filteredData.forEach(dataItem => {
            const itemId = this.getItemId(dataItem);
            const itemElement = container.querySelector(`[data-id="${itemId}"]`);
            if (itemElement) {
                itemElement.style.display = '';
            }
        });
    }

    getItemId(item) {
        // Try to get the ID from various possible fields
        return item.SongID || item.ArtistID || item.UnitID || item.CategoryID || item.id;
    }

    // Method to update data (useful for dynamic updates)
    updateData(newData) {
        this.data = newData;
        this.filteredData = [...this.data];
        this.filterAndRender();
    }
}

// Fuzzy matching function (adapted from fuzzy-search.js)
function fuzzyMatch(text, query) {
    if (!text || !query) return 0;

    const textLower = text.toLowerCase();
    const queryLower = query.toLowerCase();

    // Exact match gets highest score
    if (textLower.includes(queryLower)) {
        return 100 - (textLower.length - queryLower.length);
    }

    // Fuzzy matching - check if all query characters appear in order
    let textIndex = 0;
    let queryIndex = 0;

    while (textIndex < textLower.length && queryIndex < queryLower.length) {
        if (textLower[textIndex] === queryLower[queryIndex]) {
            queryIndex++;
        }
        textIndex++;
    }

    // If all query characters were found, calculate score
    if (queryIndex === queryLower.length) {
        return Math.max(0, 50 - (textLower.length - queryLower.length));
    }

    return 0;
}

// Enhanced SearchFilter with fuzzy matching
class FuzzySearchFilter extends SearchFilter {
    constructor(options) {
        super(options);
        this.fuzzyThreshold = options.fuzzyThreshold || 10; // Minimum score for fuzzy matches
    }

    matchesSearch(item) {
        if (!this.currentSearchTerm) return true;

        return this.searchFields.some(fieldPath => {
            const value = this.getNestedValue(item, fieldPath);
            if (!value) return false;

            if (Array.isArray(value)) {
                return value.some(arrayItem => {
                    if (typeof arrayItem === 'string') {
                        return fuzzyMatch(arrayItem, this.currentSearchTerm) >= this.fuzzyThreshold;
                    } else if (arrayItem && typeof arrayItem === 'object') {
                        return (arrayItem.NameOriginal && fuzzyMatch(arrayItem.NameOriginal, this.currentSearchTerm) >= this.fuzzyThreshold) ||
                               (arrayItem.NameEnglish && fuzzyMatch(arrayItem.NameEnglish, this.currentSearchTerm) >= this.fuzzyThreshold);
                    }
                    return false;
                });
            } else {
                return fuzzyMatch(value, this.currentSearchTerm) >= this.fuzzyThreshold;
            }
        });
    }
}