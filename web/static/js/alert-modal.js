/**
 * Alert and Confirm modals.
 * 
 * Usage:
 * 
 *     modalAlert({message: "Hello world!"}).then(callback);
 *     modalConfirm({message: "Are you sure?"}).then(callback);
 *     modalPrompt({message: "Write an answer:"}).then(callback);
 * 
 * Available options for modalAlert:
 * - message
 * - title: Alert
 * 
 * Available options for modalConfirm:
 * - message
 * - title: Confirm
 * - buttons: ["Ok", "Cancel"]
 * - event (pass `event` for easy inline onclick handlers)
 * - element (pass `this` for easy inline onclick handlers)
 * 
 * Available options for modalPrompt:
 * - message
 * - title: Prompt
 * - buttons: ["Ok", "Cancel"]
 * - type: one of "text", "number", "email", "date", or special ones "textarea" or "select"
 * - placeholder: for the input or textarea field.
 * - options: for 'select' types, this is an array of {value, label} objects.
 * 
 * Support for automatic form submission confirmation prompts:
 * 
 * On your form's Submit button:
 * - Add "gosocial-confirm-submit" to the button's classes.
 * - Include a `data-confirm="message"` attribute to the submit button.
 * The button will then block the form submission until the user confirms the modal,
 * for a relatively easy drop-in replacement to `onclick="return window.confirm()"`
 */
document.addEventListener('DOMContentLoaded', () => {
    const $modal = document.querySelector("#gosocial-alert-modal"),
        $ok = $modal.querySelector("button.gosocial-alert-ok-button"),
        $cancel = $modal.querySelector("button.gosocial-alert-cancel-button"),
        $title = $modal.querySelector("#gosocial-alert-modal-title"),
        $body = $modal.querySelector("#gosocial-alert-modal-body"),
        $promptInput = $modal.querySelector("#gosocial-alert-modal-input"),
        $promptTextArea = $modal.querySelector("#gosocial-alert-modal-textarea"),
        $promptSelectDiv = $modal.querySelector("#gosocial-alert-modal-select"),
        $promptSelect = $promptSelectDiv.getElementsByTagName("select")[0],
        alertIcon = `<i class="fa fa-exclamation-triangle mr-2"></i>`,
        confirmIcon = `<i class="fa fa-question-circle mr-2"></i>`,
        cls = 'is-active';
    
    // Current caller's promise, and (for prompts) a pointer to the input field.
    var currentPromise = null,
        currentInputField = null;
    
    const hideModal = () => {
        currentPromise = null;
        $modal.classList.remove(cls);
    };

    const showModal = ({
        message,
        title="Alert",
        isConfirm=false,
        buttons=["Ok", "Cancel"],

        isPrompt=false,
        promptType="text",    // or textarea, select, number, email, etc.
        promptPlaceholder="", // placeholder attribute for the input
        options=[],           // select options (value, label)
    }) => {
        // OK/Cancel buttons.
        $ok.innerHTML = buttons[0];
        $cancel.innerHTML = buttons[1];
        $cancel.style.display = (isConfirm || isPrompt) ? "" : "none";

        // Prompt form inputs.
        let inputField = null;
        if (isPrompt) {
            $promptTextArea.value = $promptInput.value = "";
            $promptTextArea.placeholder = promptPlaceholder;
            $promptInput.placeholder = promptPlaceholder;
            selectClearOptions();

            if (promptType === "textarea") {
                $promptTextArea.style.display = "";
                $promptInput.style.display = "none";
                $promptSelectDiv.style.display = "none";
                inputField = $promptTextArea;
            } else if (promptType === "select") {
                $promptSelectDiv.style.display = "";
                $promptInput.style.display = "none";
                $promptTextArea.style.display = "none";
                inputField = $promptSelect;

                // Input the select options.
                for (let option of options) {
                    let value, label;
                    if (typeof(option) === "string") {
                        value = label = option;
                    } else {
                        value = option.value;
                        label = option.label;
                    }

                    let opt = document.createElement('option');
                    opt.value = value;
                    opt.innerHTML = label;
                    $promptSelect.appendChild(opt);
                }
            } else {
                $promptInput.style.display = "";
                $promptTextArea.style.display = "none";
                $promptSelectDiv.style.display = "none";
                inputField = $promptInput;

                // Customize the type of input-prompts?
                switch (promptType) {
                    case "number":
                        $promptInput.type = "number";
                        break;
                    case "email":
                        $promptInput.type = "email";
                        break;
                    case "date":
                        $promptInput.type = "date";
                        break;
                    default:
                        $promptInput.type = "text";
                }
            }
        } else {
            $promptInput.style.display = "none";
            $promptTextArea.style.display = "none";
            $promptSelectDiv.style.display = "none";
        }

        // Strip HTML from message but allow line breaks.
        message = message.replace(/</g, "&lt;");
        message = message.replace(/>/g, "&gt;");
        message = message.replace(/\n/g, "<br>");

        $title.innerHTML = ((isConfirm || isPrompt) ? confirmIcon : alertIcon) + title;
        $body.innerHTML = message;

        // Show the modal.
        $modal.classList.add(cls);

        // Focus the OK button, e.g. so hitting Enter doesn't accidentally (re)click the same
        // link/button which prompted the alert box in the first place.
        window.requestAnimationFrame(() => {
            if (inputField !== null) {
                inputField.focus();
            } else {
                $ok.focus();
            }
        });

        // Return as a promise.
        return new Promise((resolve, reject) => {
            currentPromise = resolve;
            currentInputField = inputField;
        });
    };

    // Click events for the modal buttons.
    $ok.addEventListener('click', (e) => {
        if (currentPromise !== null) {
            if (currentInputField !== null) {
                currentPromise(currentInputField.value);
            } else {
                currentPromise();
            }
        }
        hideModal();
    });
    $cancel.addEventListener('click', (e) => {
        hideModal();
    });

    // Key bindings to dismiss the modal.
    window.addEventListener('keydown', (e) => {
        if ($modal.classList.contains(cls)) {
            if (e.key == 'Enter') {
                // Do not submit if the current modal has a focused textarea.
                if (currentInputField === $promptTextArea && document.activeElement === currentInputField) {
                    return;
                }
                $ok.click();
            } else if (e.key == 'Escape') {
                $cancel.click();
            }
        }
    });

    // Inline submit button confirmation prompts, e.g.: many submit buttons have name="intent"
    // and want the user to confirm before submitting, and had inline onclick handlers.
    const bindConfirmSubmit = () => {
        (document.querySelectorAll('.gosocial-confirm-submit') || []).forEach(button => {
            const message = button.dataset.confirm;
            if (!message) return;

            // Only process this node once.
            if (button.dataset.hasConfirmSubmitEnhancement) {
                return;
            } else {
                button.dataset.hasConfirmSubmitEnhancement = true;
            }

            let confirmed = false,
                submitted = false;
    
            const onclick = (e) => {
                if (confirmed && !submitted) {
                    submitted = true;
                    return;
                } else if (submitted) {
                    e.preventDefault(); // prevent double-post
                    return;
                }
    
                // Default behavior = block the form and prompt to confirm.
                e.preventDefault();
                modalConfirm({
                    message: message.replace(/\\n/g, '\n'),
                }).then(() => {
                    window.requestAnimationFrame(() => {
                        confirmed = true;
                        button.click();
                    });
                });
            }
    
            button.addEventListener('click', onclick);
        });
    };
    bindConfirmSubmit();
    document.addEventListener('htmx:afterSettle', (e) => {
        bindConfirmSubmit();
    });

    // Select Prompt helpers.
    const selectClearOptions = () => {
        let i, L = $promptSelect.options.length - 1;
        for (i = L; i >= 0; i--) {
            $promptSelect.remove(i);
        }
    }

    // Exported global functions to invoke the modal.
    window.modalAlert = async ({ message, title="Alert" }) => {
        return showModal({
            message,
            title,
            isConfirm: false,
        });
    };
    window.modalConfirm = async ({ message, title="Confirm", buttons=["Ok", "Cancel"] }) => {
        return showModal({
            message,
            title,
            isConfirm: true,
            buttons,
        });
    };
    window.modalPrompt = async ({ message, title="Prompt", buttons=["Ok", "Cancel"], type="text", placeholder="", options=[] }) => {
        return showModal({
            message,
            title,
            isPrompt: true,
            buttons,
            promptType: type,
            promptPlaceholder: placeholder,
            options,
        })
    }
});
