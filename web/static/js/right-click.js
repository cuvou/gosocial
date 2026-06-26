// Right-click button handler, to dissuade downloading.
document.addEventListener('DOMContentLoaded', () => {

    // Whitelist of paths not to apply this to.
    const whitelist = [
        '/photo/media',
        '/admin/photo/certification',
        '/admin/feedback',
    ];
    for (let path of whitelist) {
        if (window.location.pathname.indexOf(path) === 0) {
            return;
        }
    }

    const $modal = document.querySelector("#rightclick-modal"),
        $button = $modal.querySelector("button"),
        $adminLine = $modal.querySelector(".gosocial-rc-admin"),
        $lastURL = $adminLine?.getElementsByTagName("a")[0],
        cls = 'is-active';

    // Common right-click handler func.
    const onRightClick = (e) => {
        onImageSrc(e.target);
        $modal.classList.add(cls);
        e.preventDefault();
    };
    const onDragStart = (e) => {
        e.preventDefault();
        return false;
    };
    const onImageSrc = (node) => {
        if (!$lastURL) return;
        let gif = node.querySelector("source");
        let url = (node.tagName === 'VIDEO' && gif) ? gif.src : node.src
        $lastURL.href = url;
        $adminLine.style.display = url ? '' : 'none';
    };

    // (Re)Apply the handler to all images.
    const setRightClickHandlers = () => {

        // Context menu handlers.
        (document.querySelectorAll('img, video, #detailImg') || []).forEach(node => {
            // In case the img already has the handler, remove it first.
            // e.g.: HTMX lazy loaded images will need the handler re-applied after load.
            node.removeEventListener('contextmenu', onRightClick);
            node.addEventListener('contextmenu', onRightClick);
        });

        // Make images not draggable.
        (document.querySelectorAll('img') || []).forEach(node => {
            node.removeEventListener('dragstart', onDragStart);
            node.addEventListener('dragstart', onDragStart);
            node.setAttribute("draggable", "false");
        });
    };

    // Apply the right-click handlers now + re-apply on HTMX lazy loads.
    setRightClickHandlers();
    document.addEventListener('htmx:afterSettle', (e) => {
        setRightClickHandlers();
    });

    $button.addEventListener('click', () => {
        $modal.classList.remove(cls);
    });

    // Detect other key combinations that would open a context menu.
    let shiftKey = false;
    (['keydown', 'keyup']).forEach(name => {
        document.addEventListener(name, (e) => {
            shiftKey = e.shiftKey;
        });
    });
    document.addEventListener('mousedown', (e) => {
        let isRight = false;
        if ("which" in e) {
            isRight = e.which === 3;
        } else if ("button" in e) {
            isRight = e.button === 2;
        }

        if (isRight && shiftKey) {
            shiftKey = isRight = false;
            $modal.classList.add(cls);
            e.preventDefault();
        }
    });
});
