// Markdown editor widget script.
// Used by the templates/partials/markdown_editor.html component.
document.addEventListener('DOMContentLoaded', (e) => {

    // Render a Markdown preview.
    const refreshPreview = async (text, galleryLinks) => {
        return new Promise((resolve, reject) => {
            fetch("/v1/markdown", {
                method: "POST",
                mode: "same-origin",
                cache: "no-cache",
                credentials: "same-origin",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    "text": text,
                    "galleryLinks": galleryLinks,
                }),
            })
            .then(response => response.json())
            .then(data => {
                if (data.StatusCode !== 200) {
                    modalAlert({message: data.data.error});
                    return;
                }

                resolve(data.data.html);
            })
            .catch(err => {
                reject(err);
            });
        });
    };

    // Function to initialize Markdown editors.
    // Called on page load and callable later for HTMX widgets.
    window.InitializeMarkdownEditors = () => {

        // Find the Markdown widgets on this page.
        (document.querySelectorAll(".gosocial-markdown-editor") || []).forEach(editor => {
            let $tabs = editor.querySelector(".tabs"),
                $writeTab = $tabs.querySelector(".gosocial-markdown-editor-write"),
                $previewTab = $tabs.querySelector(".gosocial-markdown-editor-preview");
            let $writePanel = editor.querySelector(".gosocial-markdown-editor-write-panel"),
                $textarea = $writePanel.getElementsByTagName("textarea")[0],
                $previewPanel = editor.querySelector(".gosocial-markdown-editor-preview-panel"),
                $previewContent = $previewPanel.querySelector(".content");

            // Only process this node once.
            if (editor.dataset.hasBeenEnhanced) {
                return;
            } else {
                editor.dataset.hasBeenEnhanced = true;
            }

            // Special options.
            let galleryLinks = $textarea.dataset.galleryLinks;

            // Tab toggle function.
            const setTab = (tab) => {
                if (tab === $writeTab) {
                    $writeTab.classList.add('is-active');
                    $previewTab.classList.remove('is-active');
                }
            };

            // Configure the tab click event handlers.
            $writeTab.classList.add('is-active');
            $previewPanel.style.display = "none";
            $writeTab.childNodes[0].addEventListener('click', (e) => {
                e.preventDefault();

                // Toggle the tab visibility.
                $writeTab.classList.add('is-active');
                $writePanel.style.display = "";
                $previewTab.classList.remove('is-active');
                $previewPanel.style.display = "none";
            });
            $previewTab.childNodes[0].addEventListener('click', (e) => {
                e.preventDefault();

                // Toggle the tab visibility.
                $previewTab.classList.add('is-active');
                $previewPanel.style.display = "";
                $writeTab.classList.remove('is-active');
                $writePanel.style.display = "none";

                // Refresh the preview on tab entry.
                $previewContent.innerHTML = `<em><i class="fa fa-spinner fa-spin"></i> Loading...</em>`;
                refreshPreview($textarea.value, galleryLinks).then(html => {
                    $previewContent.innerHTML = html;
                }).catch(err => {
                    modalAlert({
                        message: err,
                    });
                });
            });
        });
    };

    // Run immediately.
    window.InitializeMarkdownEditors();

    // And after HTMX widgets settle.
    document.addEventListener('htmx:afterSettle', (e) => {
        InitializeMarkdownEditors();
    });
});