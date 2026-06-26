// gosocial inline "Quote" and "Reply" buttons that activate the comment field
// on the current page. Common logic between forum threads and photo comments.

document.addEventListener('DOMContentLoaded', function() {

    const enhance = () => {
        const $message = document.querySelector("#message");

        // Enhance the in-post Quote and Reply buttons to activate the reply field
        // at the page header instead of going to the dedicated comment page.
        (document.querySelectorAll(".gosocial-quote-button") || []).forEach(node => {

            // Only process this node once.
            if (node.dataset.hasBeenEnhanced) {
                return;
            } else {
                node.dataset.hasBeenEnhanced = true;
            }

            const message = node.dataset.quoteBody,
                replyTo = node.dataset.replyTo,
                commentID = node.dataset.commentId;

            // If we have a comment ID, have the at-mention link to it.
            let atMention = "@" + replyTo;
            if (commentID) {
                atMention = `[@${replyTo}](/go/comment?id=${commentID})`;
            }

            node.addEventListener("click", (e) => {
                e.preventDefault();

                if (replyTo) {
                    $message.value += atMention + "\n\n";
                }

                // Prepare the quoted message.
                var lines = [];
                for (let line of message.split("\n")) {
                    lines.push("> " + line);
                }

                $message.value += lines.join("\n") + "\n\n";
                $message.scrollIntoView();
                $message.focus();
            });
        });

        (document.querySelectorAll(".gosocial-reply-button") || []).forEach(node => {
            const replyTo = node.dataset.replyTo,
                commentID = node.dataset.commentId;

            // If we have a comment ID, have the at-mention link to it.
            let atMention = "@" + replyTo;
            if (commentID) {
                atMention = `[@${replyTo}](/go/comment?id=${commentID})`;
            }

            node.addEventListener("click", (e) => {
                e.preventDefault();
                $message.value += atMention + "\n\n";
                $message.scrollIntoView();
                $message.focus();
            });
        });
    };

    // Enhance comment forms now, and on HTMX updates.
    enhance();
    document.addEventListener('htmx:afterSettle', (e) => {
        enhance();
    });

});