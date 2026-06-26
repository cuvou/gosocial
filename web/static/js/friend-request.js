// Friend request "Leave a message" custom modal support.
document.addEventListener('DOMContentLoaded', () => {
    const $friendButton = document.getElementById("friendRequestButton"),
        $friendMessage = document.getElementById("friendRequestMessage"),
        friendStatus = $friendButton?.dataset.friendshipStatus,
        friendUsername = $friendButton?.dataset.username;

    if (!$friendButton) return;

    // Common onclick handler for the button, which may show a modal.
    const onclick = async (e) => {
        e.preventDefault();

        switch (friendStatus) {
            // Already a friend? = show confirmation to unfriend.
            case "approved":
                return modalConfirm({
                    title: "Remove Friend",
                    message: `You are already friends with @${friendUsername}.\n\n` +
                        "Do you want to remove this friendship?",
                    buttons: ["Remove Friendship", "Cancel"],
                }).then(callback);

            // Not yet requested? = show the send request + leave a message modal.
            case "none":
                return modalPrompt({
                    title: "Send a Friend Request",
                    message: `Would you like to add @${friendUsername} to your friend list?\n\n` +
                        `Friending @${friendUsername} will allow them to view your profile page and see any "friends-only" photos ` +
                        "that you may have on your gallery.\n\n" +
                        "You may also include an (optional) introduction message with your friend request. It " +
                        "is highly recommended to include a message, as friend requests with messages " +
                        `attached will be prioritized over ones without, and may boost your odds of @${friendUsername} ` +
                        "accepting your request!",
                    type: "textarea",
                    placeholder: "Write a nice introduction message to go along with this friend request.",
                    buttons: ["Send Friend Request", "Cancel"],
                }).then(answer => {
                    // Set the message and submit.
                    $friendMessage.value = answer;
                    callback();
                });

            // A reverse friend request is pending?
            case "requested":
                return modalConfirm({
                    title: "Accept Friend Request",
                    message: `Would you like to accept the friend request from @${friendUsername}?\n\n` +
                        `Friending @${friendUsername} will allow them to view your profile page and see any "friends-only" photos ` +
                        "that you may have on your gallery.",
                    buttons: ["Accept Friend Request", "Cancel"],
                }).then(callback);

            default:
                callback();
        }
    };

    // Common callback after a modal has been confirmed: it removes
    // the event listener and re-clicks the button to submit the form.
    const callback = () => {
        $friendButton.removeEventListener("click", onclick);
        window.requestAnimationFrame(() => {
            $friendButton.click();
        })
    };

    // Bind the onclick handler for the modal to appear.
    $friendButton.addEventListener("click", onclick);
});