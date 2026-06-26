/*
TypeAhead Search for the World Cities database.

Assumptions: an input field with id="wcs" (World City Search) exists on the page.

Embed this script on any page with a World City type-ahead search like so:

    <!-- Typeahead search for City -->
    <link rel="stylesheet" href="/static/css/typeahead.css?build={{.BuildHash}}">
    <link rel="stylesheet" href="/static/css/awesomplete-1.1.7.css">
    <script type="text/javascript" src="/static/js/awesomplete-1.1.7.min.js"></script>
    <script type="text/javascript" src="/static/js/typeahead-worldcities.js?build={{.BuildHash}}"></script>

The latest Awesomplete search result is cached globally on window.TypeAheadWorldCitiesCachedResult.

Pages that want to access it (for 'awesomplete-select' event handlers) can get the result from
there. For example on /web/templates/events/edit.html.
*/

// Globally accessible cached result of the latest World Cities type-ahead search.
window.TypeAheadWorldCitiesCachedResult = [];

document.addEventListener('DOMContentLoaded', () => {

    // Type-ahead search for World Cities.
    (function() {
        const $input = document.getElementById("wcs");

        const autocomplete = new Awesomplete($input, {
            minChars: 1,
            maxItems: 7,
            autoFirst: true,
            tabSelect: true,
            sort: false, // disable built-in sorting for our custom ranking algorithm below
            list: [],
            filter(suggestion, input) {
                let words = input.toLowerCase().split(/\s+/).filter(Boolean);

                // Get both the Canonical and CanonicalAscii for matching.
                let canonical = suggestion.label.toLowerCase();
                let ascii = suggestion.value.toLowerCase();

                return words.every(word => canonical.includes(word) || ascii.includes(word));
            },
            replace(suggestion) {
                // Use the Label (with e.g. accents/umlauts) as the final selection instead
                // of the Value (where we stored the AsciiCanonical but we don't want it as the value).
                this.input.value = suggestion.label;
            },
        });

        $input.addEventListener('input', async function () {
            const query = encodeURIComponent(this.value);
            const res = await fetch(`/v1/world-cities?query=${query}`);
            const data = await res.json();

            const cleanInput = this.value.trim().toLowerCase();

            /*
            Massage the data from backend to sort by relevance before passing
            it onward to Awesomeplete.

            Specific examples:

            - 'Zürich, Zürich, CH' was difficult to search for because we have at least 44 cities in
              Zürich and the simple word-based tokenizer search wasn't able to filter out the irrelevant
              cities even if you searched for that full literal string (since all 44 cities matched both
              the terms Zürich and CH).
            - Meanwhile, a search for 'Rome, IT' must still be possible despite the database having it
              spelled out as 'Rome, Lazio, IT' which the word-based tokenizer matching was ideal for.
            */
            data.sort((a, b) => {
                let aAscii = a.CanonicalAscii.toLowerCase();
                let bAscii = b.CanonicalAscii.toLowerCase();
                let aCity = a.City.toLowerCase();
                let bCity = b.City.toLowerCase();

                // Tier 1: Exact city name match (e.g., typing "Zurich" perfectly matches city "Zurich")
                let aExactCity = (aCity === cleanInput) ? 1 : 0;
                let bExactCity = (bCity === cleanInput) ? 1 : 0;
                if (aExactCity !== bExactCity) return bExactCity - aExactCity;

                // Tier 2: Exact full string match (unlikely to tie, but great for exact copy-pastes)
                let aExactFull = (aAscii === cleanInput) ? 1 : 0;
                let bExactFull = (bAscii === cleanInput) ? 1 : 0;
                if (aExactFull !== bExactFull) return bExactFull - aExactFull;

                // Tier 3: Starts with the input (e.g., "Zurich..." comes before "...Zurich...")
                let aStarts = aAscii.startsWith(cleanInput) ? 1 : 0;
                let bStarts = bAscii.startsWith(cleanInput) ? 1 : 0;
                if (aStarts !== bStarts) return bStarts - aStarts;

                // Tier 4: Fallback to string length (shorter strings mean less "noise" matching, prioritizing major hubs)
                return aAscii.length - bAscii.length;
            });

            // Export the cached result to the global window namespace, so e.g. pages
            // that want to bind an event handler can operate on the result.
            window.TypeAheadWorldCitiesCachedResult = data;

            /*
            Assign the massaged and sorted list to Awesomplete.

            Notes:

            - Label is shown on front-end and is what we want in the text box when selected.
            - Value will store the ASCII version for nicer matching (we use it in the filter() above).
            - Awesomplete would normally place the Value in our text box but our custom replace() function
              will have it paste the Label in instead.
            */
            autocomplete.list = data.map(item => ({
                label: item.Canonical,
                value: item.CanonicalAscii,
            }));
        });
    })();
});