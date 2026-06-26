/* gosocial service worker, for web push notifications */

self.addEventListener('install', (event) => {
    self.skipWaiting();
});

self.addEventListener('push', (event) => {
    const payload = JSON.parse(event.data.text());
    try {
        event.waitUntil(
            self.registration.showNotification(payload.title, {
                body: payload.body,
                icon: "/static/img/favicon-192.png",
            })
        );
    } catch(e) {
        console.error("sw.showNotification:", e);
    }
});
