/* gosocial web push notification helper */
navigator.serviceWorker.register("/sw.js", {
    scope: "/",
}).catch(err => {
    console.error("Service Worker NOT registered:", err);
});

function PushNotificationSubscribe() {
    navigator.serviceWorker.ready.then(async function(registration) { 
        return registration.pushManager.getSubscription().then(async function(subscription) {
            // If a subscription was already found, return it.
            if (subscription) {
                return subscription;
            }
    
            // Get the server's public key.
            const response = await fetch("/v1/web-push/vapid-public-key");
            const vapidPublicKey = await response.text();
    
            // Subscribe the user.
            return registration.pushManager.subscribe({
                userVisibleOnly: true,
                applicationServerKey: vapidPublicKey,
            });
        }).then(subscription => {
    
            // Post it to the backend.
            const serialized = JSON.stringify(subscription);
            fetch("/v1/web-push/register", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: serialized
            });
        });
    });
}

// If the user has already given notification permission, (re)subscribe for push.
document.addEventListener("DOMContentLoaded", e => {
    if (Notification.permission === "granted") {
        PushNotificationSubscribe();
    }
});
