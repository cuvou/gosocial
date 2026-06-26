// Bulma CSS fixes and features for gosocial.
document.addEventListener('DOMContentLoaded', () => {

  // Make all off-site hyperlinks open in a new tab.
  (document.querySelectorAll("a") || []).forEach(node => {
    let href = node.attributes.href;
    let target = node.attributes.target;
    if (href === undefined || target) return;
    href = href.textContent;
    if (href.indexOf("http:") === 0 || href.indexOf("https:") === 0) {
      node.target = "_blank";
    }
  });

  // If we are a PWA in standalone mode, clicking on-site links shows our custom loading spinner.
  window.IsWebApp = false;
  if (window.matchMedia('(display-mode: standalone)').matches) {
    window.IsWebApp = true;
    const spinner = document.querySelector("#gosocial-pwa-loader");
    (document.querySelectorAll("a, form") || []).forEach(node => {
      // Links: only on-site ones.
      if (node.tagName === 'A') {
        let href = node.attributes.href?.textContent;
        if (!href) return;
        if (node.target === '_blank' || href.indexOf('#') === 0 || href.indexOf(location.pathname) === 0) return;
      }

      // Show our spinner on click or submit.
      node.addEventListener(node.tagName === 'A' ? 'click' : 'submit', () => {
        spinner.style.display = "block";
      });
    });
  }

  // Speaking of PWA: handle the 'beforeinstallprompt' event to catch the browser's
  // default install UI and show our own subtle banner at the top of the page.
  window.addEventListener('beforeinstallprompt', (e) => {
    let installPrompt = e,
      $installBanner = document.querySelector("#pwa-custom-install-banner"),
      $installButton = document.querySelector("#pwa-custom-install-button"),
      $hideButton = document.querySelector("#pwa-custom-install-hide"),
      localStorageKey = 'hide-pwa-install-prompt';
    if (!($installBanner && $installButton && $hideButton)) return;

    e.preventDefault();

    // Safety: if we've already detected we're a PWA, stop here.
    if (window.IsWebApp) return;

    // User has said 'never show again'?
    if (localStorage[localStorageKey]) {
      return;
    }

    // Function to dismiss and not show the banner again.
    const dismiss = () => {
      $installBanner.style.display = 'none';
      localStorage[localStorageKey] = true;
    };

    $installBanner.style.display = '';
    $installButton.addEventListener('click', async (e) => {
      if (!installPrompt) return;
      const result = await installPrompt.prompt();
      if (result.outcome === 'dismissed') {
        dismiss();
      }
      installPrompt = null;
    });
    $hideButton.addEventListener('click', (e) => {
      modalConfirm({
        message: "You will not be reminded again from this device about our web app.\n\n" +
          "Note: this setting will be remembered locally in your web browser. If you clear your " +
          "cookies later, or if you are using an incognito window/private browsing session, this " +
          "setting may be lost later and the web app reminder may appear again.\n\n" +
          "Ok to continue?",
      }).then(dismiss);
    });
  });

  // Hamburger menu script.
  (function () {
    // Get all "navbar-burger" elements
    const $navbarBurgers = Array.prototype.slice.call(document.querySelectorAll('.navbar-burger'), 0);

    // Add a click event on each of them
    $navbarBurgers.forEach(el => {
      el.addEventListener('click', () => {

        // Get the target from the "data-target" attribute
        const target = el.dataset.target;
        const $target = document.getElementById(target);

        // Toggle the "is-active" class on both the "navbar-burger" and the "navbar-menu"
        el.classList.toggle('is-active');
        $target.classList.toggle('is-active');

      });
    });
  })();

  // Mobile devices: hide the top nav bar on scroll.
  (function() {
    const topNav = document.querySelector(".navbar.is-fixed-top");
    const mainMenu = topNav.querySelector(".navbar-menu");
    let prevScrollPos = window.pageYOffset;
    window.addEventListener('scroll', (e) => {

      // Only do this for mobile/small screens.
      let isMobile = window.innerWidth < 1024;
      let isLoggedIn = window.gosocialGlobalState.isLoggedIn;
      let isTopOfPage = window.pageYOffset <= 52;

      /* Always show top nav when the hamburger menu is expanded. */
      if (!isLoggedIn || !isMobile || mainMenu.classList.contains("is-active") || isTopOfPage) {
        topNav.style.top = "0";
        return;
      }

      let currentScrollPos = window.pageYOffset;
      if (prevScrollPos < currentScrollPos) {
        topNav.style.top = "-52px";
      } else {
        topNav.style.top = "0";
      }
      prevScrollPos = currentScrollPos;
    })
  })();

  // Allow the "More" drop-down to work on mobile (toggle is-active on click instead of requiring mouseover)
  (function () {
    const menu = document.querySelector("#navbar-more"),
      userMenu = document.querySelector("#navbar-user"),
      activeClass = "is-active";

    if (!menu) return;

    // Click the "More" menu to permanently toggle the menu.
    menu.addEventListener("click", (e) => {
      if (menu.classList.contains(activeClass)) {
        menu.classList.remove(activeClass);
      } else {
        menu.classList.add(activeClass);
      }
      e.stopPropagation();
    });

    // Touching the user drop-down button toggles it.
    if (userMenu !== null) {
      userMenu.addEventListener("touchstart", (e) => {
        // On mobile/tablet screens they had to hamburger menu their way here anyway, let it thru.
        if (screen.width < 1024) {
          return;
        }

        e.preventDefault();
        if (userMenu.classList.contains(activeClass)) {
          userMenu.classList.remove(activeClass);
        } else {
          userMenu.classList.add(activeClass);
        }
      });
    }

    // Touching a link from the user menu lets it click thru.
    (document.querySelectorAll(".navbar-dropdown") || []).forEach(node => {
      node.addEventListener("touchstart", (e) => {
        e.stopPropagation();
      });
    });

    // Clicking the rest of the body will close an active navbar-dropdown.
    (document.addEventListener("click", (e) => {
      (document.querySelectorAll(".navbar-dropdown.is-active, .navbar-item.is-active") || []).forEach(node => {
        node.classList.remove(activeClass);
      });
    }));
  })();

  // Dropdown menus.
  const enhanceDropdowns = () => {
    (document.querySelectorAll(".dropdown") || []).forEach(node => {

      // Skip Vue.js managed dropdowns.
      if (node.classList.contains("vue-managed")) {
        return;
      }

      // Only process this node once.
      if (node.dataset.hasBeenEnhanced) {
          return;
      } else {
          node.dataset.hasBeenEnhanced = true;
      }

      // If this dropdown is near the bottom of the page, make it open upwards.
      if (!node.classList.contains("is-up") && !node.classList.contains("is-right")) {
        let offsetTop = node.offsetTop,
        body = document.body,
        html = document.documentElement,
        pageHeight = Math.max(body.scrollHeight, body.offsetHeight,
          html.clientHeight, html.scrollHeight, html.offsetHeight,
        );
        if (pageHeight - offsetTop < 450) {
          node.classList.add("is-up");
        }
      }

      const button = node.querySelector("button");
      button.addEventListener("click", (e) => {
        node.classList.toggle("is-active");
      })
    });
  };
  enhanceDropdowns();

  // Common event handlers for bulma modals.
  (document.querySelectorAll(".modal-background, .modal-close, .photo-modal") || []).forEach(node => {
    const target = node.closest(".modal");
    if (target.classList.contains("vue-managed") || target.classList.contains("gosocial-important-modal")) {
      return;
    }
    node.addEventListener("click", () => {
      target.classList.remove("is-active");
    });
  });

  // Collapsible cards for mobile view (e.g. People Search filters box)
  (document.querySelectorAll(".card.gosocial-collapsible-mobile") || []).forEach(node => {
    const header = node.querySelector(".card-header"),
      body = node.querySelector(".card-content"),
      icon = header.querySelector("button.card-header-icon > .icon > i"),
      always = node.classList.contains("gosocial-collapsible-always");

    // Icon classes.
    const iconExpanded = "fa-angle-up",
      iconContracted = "fa-angle-down";

    // If we are already on mobile, hide the body now.
    if (screen.width <= 768 || always) {
      body.style.display = "none";
      if (icon !== null) {
        icon.classList.remove(iconExpanded);
        icon.classList.add(iconContracted);
      }
    }

    // Add click toggle handler to the header.
    header.addEventListener("click", () => {
      if (body.style.display === "none") {
        body.style.display = "block";
        if (icon !== null) {
          icon.classList.remove(iconContracted);
          icon.classList.add(iconExpanded);
        }
      } else {
        body.style.display = "none";
        if (icon !== null) {
          icon.classList.remove(iconExpanded);
          icon.classList.add(iconContracted);
        }
      }
    });
  });

  // Reveal all blurred images on click.
  (document.querySelectorAll(".blurred-explicit") || []).forEach(node => {
    node.addEventListener("click", e => {
      if (node.classList.contains("blurred-explicit")) {
        node.classList.remove("blurred-explicit");
        if (node.tagName !== "VIDEO") {
          e.preventDefault();
          e.stopPropagation();
        }
      }
    });

    // Video tag: autoplay is disabled when blurred, onClick doesn't fire,
    // set the handler for onPlay.
    node.addEventListener("play", e => {
      if (node.classList.contains("blurred-explicit")) {
        node.classList.remove("blurred-explicit");
      }
    });
  });

  // Deep links to comments on pages which lazy load their comments.
  // If the URL hash is a comment ID "#p1234" then scroll to the "#comments" and let
  // the HTMX widget load, then scroll to the comment ID.
  if (window.location.hash.match(/^#p(\d+)$/)) {
    const $comments = document.querySelector("#comments");
    const $preloadComment = document.querySelector(window.location.hash);

    // Only run this logic if the comments are lazy loaded and the target isn't already on the page.
    if ($comments && !$preloadComment) {
      $comments.scrollIntoView();

      // Add an HTMX binding.
      document.addEventListener('htmx:afterSettle', (e) => {
        // Find the comment now.
        const $comment = document.querySelector(window.location.hash);
        if ($comment) {
          $comment.scrollIntoView();
        } else {
          console.error("HTMX comments loaded but the comment ID was not found.");
        }
      });
    }
  }

  // Auto-hide elements on page load. For example: 'Preview' buttons for writing Markdown
  // comments on the forum and photos. We have a fancy inline preview widget, but in case
  // scripts are disabled, allow the legacy Preview button to still remain.
  (document.querySelectorAll(".gosocial-hide-on-load") || []).forEach(node => {
    node.style.display = "none";
  });

  // Rebind things on HTMX re-settle such as dropdown menus.
  document.addEventListener('htmx:afterSettle', (e) => {
      enhanceDropdowns();
  });
});
