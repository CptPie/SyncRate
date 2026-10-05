/**
 * Category multi-select filter
 *
 * Replaces the single-choice category <select> used across the browse pages and
 * the room creation forms with a checkbox dropdown, so any combination of
 * categories can be picked. "All Categories" is kept as an explicit option and
 * remains the default.
 *
 * An empty selection means "all categories" — callers get [] and should simply
 * skip the category filter rather than matching nothing.
 */
const CategoryMultiSelect = (function () {
  "use strict";

  /**
   * Build the label shown on the closed toggle button.
   * @param {Array<{id: string, name: string}>} options - All category options
   * @param {Array<string>} selected - Currently selected category IDs
   * @param {string} allLabel - Text used when nothing is selected
   * @returns {string}
   */
  function summarise(options, selected, allLabel) {
    if (selected.length === 0 || selected.length === options.length) {
      return allLabel;
    }
    if (selected.length === 1) {
      const match = options.find((option) => option.id === selected[0]);
      return match ? match.name : allLabel;
    }
    return `${selected.length} categories`;
  }

  /**
   * Build one checkbox row. Names come from the database, so they are set as
   * text rather than markup.
   * @param {string} name - Visible label
   * @param {string|null} value - Category ID, or null for the "all" row
   * @returns {HTMLLabelElement}
   */
  function buildOption(name, value) {
    const row = document.createElement("label");
    row.className = "multi-select-option";

    const box = document.createElement("input");
    box.type = "checkbox";
    if (value === null) {
      box.setAttribute("data-all-categories", "");
      row.classList.add("multi-select-all");
    } else {
      box.value = value;
    }

    const text = document.createElement("span");
    text.textContent = name;

    row.appendChild(box);
    row.appendChild(text);
    return row;
  }

  /**
   * Mount a category multi-select into a placeholder element.
   *
   * @param {HTMLElement|string} target - Element (or its id) to render into
   * @param {Object} config
   * @param {Array<Object>} config.categories - Category records; each needs a
   *     CategoryID and a Name (the shapes the handlers already serialise)
   * @param {string} [config.allLabel] - Label for the "everything" state
   * @param {string} [config.labelId] - Id of the visible field label, used for
   *     aria-labelledby; falls back to a generic aria-label when absent
   * @param {Function} [config.onChange] - Called with the selected id strings
   *     whenever the selection changes
   * @returns {{getSelected: Function, getSelectedNumbers: Function, setSelected: Function, element: HTMLElement}|null}
   */
  function mount(target, config) {
    const host =
      typeof target === "string" ? document.getElementById(target) : target;
    if (!host) {
      console.warn("CategoryMultiSelect: mount target not found", target);
      return null;
    }

    const settings = config || {};
    const allLabel = settings.allLabel || "All Categories";
    const onChange = settings.onChange;
    const options = (settings.categories || []).map((category) => ({
      id: String(category.CategoryID),
      name: category.Name,
    }));

    let selected = [];

    const toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "multi-select-toggle";
    toggle.setAttribute("aria-expanded", "false");

    // The visible field label is a plain <label> now that the control is not a
    // single form element, so point the button at it for screen readers.
    const labelId = settings.labelId || "category-filter-label";
    if (document.getElementById(labelId)) {
      toggle.setAttribute("aria-labelledby", labelId);
    } else {
      toggle.setAttribute("aria-label", "Filter by category");
    }

    const value = document.createElement("span");
    value.className = "multi-select-value";
    const caret = document.createElement("span");
    caret.className = "multi-select-caret";
    caret.setAttribute("aria-hidden", "true");
    caret.textContent = "▾";
    toggle.appendChild(value);
    toggle.appendChild(caret);

    const panel = document.createElement("div");
    panel.className = "multi-select-panel";
    panel.setAttribute("role", "group");

    const allRow = buildOption(allLabel, null);
    panel.appendChild(allRow);

    const divider = document.createElement("div");
    divider.className = "multi-select-divider";
    panel.appendChild(divider);

    const boxes = options.map((option) => {
      const row = buildOption(option.name, option.id);
      panel.appendChild(row);
      return row.querySelector("input");
    });
    const allBox = allRow.querySelector("input");

    host.classList.add("multi-select");
    host.innerHTML = "";
    host.appendChild(toggle);
    host.appendChild(panel);

    function syncDisplay() {
      value.textContent = summarise(options, selected, allLabel);
      // "All Categories" is the state of having narrowed to nothing in
      // particular, so it tracks the individual boxes rather than being set
      // independently.
      allBox.checked = selected.length === 0;
    }

    function setSelected(ids) {
      const wanted = (ids || []).map(String);
      selected = options
        .map((option) => option.id)
        .filter((id) => wanted.includes(id));
      boxes.forEach((box) => {
        box.checked = selected.includes(box.value);
      });
      syncDisplay();
    }

    function emitChange() {
      syncDisplay();
      if (typeof onChange === "function") {
        onChange(selected.slice());
      }
    }

    function open() {
      panel.classList.add("is-open");
      toggle.setAttribute("aria-expanded", "true");
    }

    function close() {
      panel.classList.remove("is-open");
      toggle.setAttribute("aria-expanded", "false");
    }

    toggle.addEventListener("click", function (event) {
      event.stopPropagation();
      if (panel.classList.contains("is-open")) {
        close();
      } else {
        open();
      }
    });

    allBox.addEventListener("change", function () {
      // Unchecking "All Categories" on its own would leave the same filter
      // behind, so either direction resolves to clearing the narrowing.
      selected = [];
      boxes.forEach((box) => {
        box.checked = false;
      });
      emitChange();
    });

    boxes.forEach((box) => {
      box.addEventListener("change", function () {
        selected = boxes
          .filter((other) => other.checked)
          .map((other) => other.value);
        emitChange();
      });
    });

    // Keep clicks inside the panel from reaching the document handler below.
    panel.addEventListener("click", function (event) {
      event.stopPropagation();
    });

    document.addEventListener("click", function (event) {
      if (!host.contains(event.target)) {
        close();
      }
    });

    host.addEventListener("keydown", function (event) {
      if (event.key === "Escape" && panel.classList.contains("is-open")) {
        close();
        toggle.focus();
      }
    });

    syncDisplay();

    return {
      element: host,
      /** @returns {Array<string>} Selected category IDs; empty means all. */
      getSelected: function () {
        return selected.slice();
      },
      /** @returns {Array<number>} Selected category IDs as numbers. */
      getSelectedNumbers: function () {
        return selected.map(Number);
      },
      setSelected: setSelected,
    };
  }

  return { mount: mount };
})();
