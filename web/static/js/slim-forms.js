// Global <form> events to prevent double posts and tidy up GET query parameters.
document.addEventListener('DOMContentLoaded', (e) => {

    // Keep track of all the forms modified.
    let allForms = [];

    /* Slim "GET" forms: an onSubmit handler that trims empty query parameters. */
    const slimmifyGetForms = (form) => {

        // Trim their empty parameters.
        form.addEventListener("submit", (e) => {
            for (let member of form.elements) {
                if (!member.value && !member.disabled) {
                    member.disabled = true;
                    member.dataset.slimFormDisabled = true;
                }
            }
        });
    };

    /* Double-post preventing "POST" forms */
    const debouncePostForms = (form) => {
        form.addEventListener("submit", (e) => {

            // Find and disable the submit buttons.
            const submitButtons = form.querySelectorAll('input[type="submit"], button:not([type]), button[type="submit"]');
            submitButtons.forEach(button => {
                if (!button.disabled) {

                    // Disable it on the next frame, so if the submit button has an important
                    // name for the form, it is still included in the form post.
                    window.requestAnimationFrame(() => {
                        button.disabled = true;
                        button.dataset.slimFormSubmitDisabled = true;
                    });

                }
            });
        });
    };

    /* Failsafes: re-enable disabled fields if the user hit Back or reloaded the page. */
    const resetForms = () => {
        for (let form of allForms) {
            const method = (form.method || "GET").toUpperCase();
            const submitButtons = form.querySelectorAll('input[type="submit"], button:not([type]), button[type="submit"]');

            // Restore form field properties.
            for (let member of form.elements) {
                if (method === "GET" && member.dataset.slimFormDisabled) {
                    member.disabled = false;
                    delete member.dataset.slimFormDisabled;
                }
            }

            // Re-enable submit buttons.
            for (let button of submitButtons) {
                if (button.dataset.slimFormSubmitDisabled) {
                    button.disabled = false;
                    delete button.dataset.slimFormSubmitDisabled;
                }
            }
        }
    };
    window.addEventListener('pageshow', resetForms);

    // Export the resetForms function globally so we can manually un-disable submit buttons where needed.
    window.SlimFormsReset = () => {
        window.requestAnimationFrame(resetForms);
    };

    // Find forms and bind events.
    (document.querySelectorAll("form") || []).forEach(form => {
        allForms.push(form);

        // How do we treat this form?
        let method = (form.method || "GET").toUpperCase();
        switch (method) {
            case "GET":
                return slimmifyGetForms(form);
            case "POST":
                return debouncePostForms(form);
        }

    });
});