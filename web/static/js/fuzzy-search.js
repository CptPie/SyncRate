// Fuzzy Search Functionality
function setupFuzzySearch(inputId, dropdownId, selectedId, hiddenInputId, data, idField, nameField, singleSelect = false) {
    const input = document.getElementById(inputId);
    const dropdown = document.getElementById(dropdownId);
    const selectedContainer = document.getElementById(selectedId);
    const hiddenInput = document.getElementById(hiddenInputId);

    let selectedItems = [];

    function updateHiddenInput() {
        if (singleSelect) {
            hiddenInput.value = selectedItems.length > 0 ? selectedItems[0].id : '';
        } else {
            hiddenInput.value = selectedItems.map(item => item.id).join(',');
        }
    }

    function renderSelectedItems() {
        selectedContainer.innerHTML = '';
        selectedItems.forEach((item, index) => {
            const tag = document.createElement('div');
            tag.className = 'selected-tag';
            tag.innerHTML = `
                <span>${item.name}</span>
                <button type="button" class="remove-tag" data-index="${index}">×</button>
            `;
            selectedContainer.appendChild(tag);
        });

        // Add event listeners for remove buttons
        selectedContainer.querySelectorAll('.remove-tag').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const index = parseInt(e.target.dataset.index);
                selectedItems.splice(index, 1);
                renderSelectedItems();
                updateHiddenInput();
            });
        });
    }

    function fuzzyMatch(text, query) {
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

    function filterAndRenderDropdown(query) {
        if (!query.trim()) {
            dropdown.style.display = 'none';
            return;
        }

        // Filter and score items
        const filteredItems = data
            .map(item => ({
                ...item,
                score: fuzzyMatch(item[nameField], query)
            }))
            .filter(item => item.score > 0)
            .sort((a, b) => b.score - a.score)
            .slice(0, 10); // Limit to top 10 results

        if (filteredItems.length === 0) {
            dropdown.style.display = 'none';
            return;
        }

        dropdown.innerHTML = '';
        filteredItems.forEach(item => {
            const option = document.createElement('div');
            option.className = 'fuzzy-option';
            option.textContent = item[nameField];
            option.dataset.id = item[idField];
            option.dataset.name = item[nameField];

            option.addEventListener('click', () => {
                const itemData = {
                    id: item[idField],
                    name: item[nameField]
                };

                // Check if item is already selected
                if (selectedItems.some(selected => selected.id === itemData.id)) {
                    return;
                }

                if (singleSelect) {
                    selectedItems = [itemData];
                } else {
                    selectedItems.push(itemData);
                }

                renderSelectedItems();
                updateHiddenInput();
                input.value = '';
                dropdown.style.display = 'none';
            });

            dropdown.appendChild(option);
        });

        dropdown.style.display = 'block';
    }

    // Event listeners
    input.addEventListener('input', (e) => {
        filterAndRenderDropdown(e.target.value);
    });

    input.addEventListener('focus', (e) => {
        if (e.target.value.trim()) {
            filterAndRenderDropdown(e.target.value);
        }
    });

    // Hide dropdown when clicking outside
    document.addEventListener('click', (e) => {
        if (!input.contains(e.target) && !dropdown.contains(e.target)) {
            dropdown.style.display = 'none';
        }
    });

    // Keyboard navigation
    input.addEventListener('keydown', (e) => {
        const options = dropdown.querySelectorAll('.fuzzy-option');
        const activeOption = dropdown.querySelector('.fuzzy-option.active');
        let activeIndex = -1;

        if (activeOption) {
            activeIndex = Array.from(options).indexOf(activeOption);
        }

        switch (e.key) {
            case 'ArrowDown':
                e.preventDefault();
                if (options.length > 0) {
                    if (activeOption) activeOption.classList.remove('active');
                    activeIndex = (activeIndex + 1) % options.length;
                    options[activeIndex].classList.add('active');
                }
                break;

            case 'ArrowUp':
                e.preventDefault();
                if (options.length > 0) {
                    if (activeOption) activeOption.classList.remove('active');
                    activeIndex = activeIndex <= 0 ? options.length - 1 : activeIndex - 1;
                    options[activeIndex].classList.add('active');
                }
                break;

            case 'Enter':
                e.preventDefault();
                if (activeOption) {
                    activeOption.click();
                }
                break;

            case 'Escape':
                dropdown.style.display = 'none';
                input.blur();
                break;
        }
    });
}