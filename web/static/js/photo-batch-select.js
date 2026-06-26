/*
Batch Edit controls for photo gallery views.

Used on:
- Site Gallery (for admin view)
- User Gallery (for owner or admin view)
- My Media page

Expectations:
- Checkboxes with a class "gosocial-edit-photo-id"
- Buttons with class "gosocial-select-all" and "gosocial-select-none"
- A div with class "gosocial-count-selected" that shows the current count
- Submit buttons with class "gosocial-edit-buttons"
*/
document.addEventListener("DOMContentLoaded", () => {
    const checkboxes = document.getElementsByClassName("gosocial-edit-photo-id"),
        $checkAll = document.querySelector("#gosocial-select-all"),
        $checkNone = document.querySelector("#gosocial-select-none"),
        $countSelected = document.querySelector("#gosocial-count-selected"),
        $submitButtons = document.querySelector("#gosocial-edit-buttons");

    $submitButtons.style.display = "none";

    const setAllChecked = (v) => {
        for (let box of checkboxes) {
            box.checked = v;
        }
    };

    const areAnyChecked = () => {
        let any = false,
            count = 0;
        for (let box of checkboxes) {
            if (box.checked) {
                any = true;
                count++;
            }
        }

        // update the selected count
        $countSelected.innerHTML = count > 0 ? `${count} selected.` : "";
        $countSelected.style.display = count > 0 ? "" : "none";
        return any;
    };

    const showHideButtons = () => {
        $submitButtons.style.display = areAnyChecked() ? "" : "none";
    };
    showHideButtons();

    // Check/Uncheck All buttons.
    $checkAll.addEventListener("click", (e) => {
        setAllChecked(true);
        showHideButtons();
    });
    $checkNone.addEventListener("click", (e) => {
        setAllChecked(false);
        showHideButtons();
    });

    // When checkboxes are toggled.
    for (let box of checkboxes) {
        box.addEventListener("change", (e) => {
            showHideButtons();
        });
    }
});