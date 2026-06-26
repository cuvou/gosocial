// Like button handler.
document.addEventListener('DOMContentLoaded', () => {
    const red = "has-text-danger";
    let busy = false;

    // Globally export the "Like" API call (e.g. for fancy lightbox modal on gallery)
    window.gosocialV1Likes = async function({ tableName="photos", tableID, liking=true }) {
        return new Promise(async (resolve, reject) => {
            return fetch("/v1/likes", {
                method: "POST",
                mode: "same-origin",
                cache: "no-cache",
                credentials: "same-origin",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    "name": tableName, // TODO
                    "id": ""+tableID,
                    "unlike": !liking,
                    "page": window.location.pathname + window.location.search + window.location.hash,
                }),
            })
            .then((response) => response.json())
            .then((data) => {
                if (data.StatusCode !== 200) {
                    modalAlert({message: data.data.error});
                    return;
                }

                // Pass to caller.
                resolve(data);
            }).catch(resp => {
                reject(resp);
            });
        });
    };

    // Bind to the like buttons.
    const enhance = () => {
        (document.querySelectorAll(".gosocial-like-button") || []).forEach(node => {

            // Only process this node once.
            if (node.dataset.hasBeenEnhanced) {
                return;
            } else {
                node.dataset.hasBeenEnhanced = true;
            }

            node.addEventListener("click", (e) => {
                e.preventDefault();
                if (busy) return;

                let $icon = node.querySelector(".icon"),
                    $label = node.querySelector(".gosocial-likes"),
                    tableName = node.dataset.tableName,
                    tableID = node.dataset.tableId,
                    liking = false;

                // Toggle the color of the heart.
                if ($icon.classList.contains(red)) {
                    $icon.classList.remove(red);
                } else {
                    liking = true;
                    $icon.classList.add(red);
                }

                // Ajax request to backend.
                busy = true;
                return gosocialV1Likes({
                    tableName,
                    tableID,
                    liking,
                }).then(data => {
                    let likes = data.data.likes;
                    if (likes === 0) {
                        $label.innerHTML = "Like";
                    } else {
                        $label.innerHTML = `Like (${likes})`;
                    }
                }).finally(() => {
                    busy = false;
                });
            });
        });
    }

    // Enhance comment forms now, and on HTMX updates.
    enhance();
    document.addEventListener('htmx:afterSettle', (e) => {
        enhance();
    });
});
