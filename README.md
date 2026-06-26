# GoSocial: A social networking website

**GoSocial** is a complete social networking website written in Go. It is a pared
down, 'white labeled' version of a successful production website which was
launched in August 2022 and grew to a community of 12,000 members by June 2026.

> **Live Demo:** The fully featured branch of GoSocial has a live demo instance
> at the following link:
>
> **[GoSocial.Cuvou.com](https://gosocial.cuvou.com)**
>
> You can sign up a test account with a random fake e-mail address and look
> around! Test accounts are deleted automatically after a while.

* [Setup Guide](#setup-guide)
* [Features](#features)
* [Tech Stack](#tech-stack)
* [Dependencies](#dependencies)
* [Building the App](#building-the-app)
* [Configuring](#configuring)
* [Usage](#usage)
* [Create Admin User Accounts](#create-admin-user-accounts)
* [Docker-Compose](#docker-compose)
* [A Brief Tour of the Code](#a-brief-tour-of-the-code)
* [Cron Workers](#cron-workers)
* [License](#license)

The release of the GoSocial source code comes in a couple of flavors:

* The **Free & Open Source Community Edition** includes the common baseline
  features that _any_ social community website would need: profiles, friends,
  messages, forums, photo gallery, chat room, etc.

  This edition is released under the GNU General Public License as a free &
  open source project with 'copyleft' licensing terms. You are free to take
  this codebase and adapt it for your own needs, but any modifications you
  make to its code must also be released as open source software that your
  users have access to.
* The **Paid & Licensed Editions** bridge the gap between those basic
  features to _all the features_ from the upstream website that this codebase
  was branched from.

  These editions are suitable for use in closed source, commercial products.
  Please contact the author to discuss pricing and your project's needs.

## Setup Guide

See the [SetupGuide.md](SetupGuide.md) for detailed instructions how to deploy
GoSocial onto a production web server.

## Features

The **Free & Open Source Edition** comes with all of the basic features that
_any_ social networking website would need to be basically useful.

Some of these features include:

* **User Accounts**
  * Sign up (w/ optional e-mail verification), Log in, Forgotten Password
    workflows (e-mailed links to reset passwords).
  * Two-Factor Auth supported (TOTP).
  * Profile Pages, with custom theme support (background/header images,
    colors). Essay fields like 'About Me' and basic fields like location,
    age and gender.
  * Login Sessions: users can see the devices they're logged in from and
    remotely log out other devices.
  * Compliance with GDPR and similar privacy laws: users may deeply delete
    their account and all their media and data, and user data export to ZIP
    file supported.
* **Social Features**
  * Friend Requests & Follows. Users can mark some of their media as 'friends
    only' or they may Follow people and be notified about public media they
    share.
  * Likes & Comments.
  * On-site Notifications & Web Push Notifications supported.
  * Direct Messages with on-site inboxes.
  * A "Member Directory" to browse and search for profiles on the site.
  * "Who's Nearby?" to sort members on the directory by their distance to you,
    or by their proximity to a major city you search for.
* **Forums**
  * The admin can create a handful of Forum boards around various topics and
    users can create threads and replies under each forum.
  * Polls may be added to forum threads to collect votes from other users.
* **Photo Galleries**
  * Each user has their own Photo Gallery that they can upload media to.
  * There is a Site-wide Photo Gallery which shows photos from the community
    all in one place. Users opt in photos to be featured on the Site Gallery.
  * Photo visibility options include "Public" (any logged-in user can see it),
    "Friends only" (only your friends can see it), or "Private" (you selectively
    unlock your private content for others on a per-person basis, with ability
    to revoke that share from one or all easily).
  * Animated GIFs supported: uploaded GIF images are encoded to small .mp4
    video clips using ffmpeg, for file size/bandwidth/performance reasons.
  * Media Quotas: set a file size limit for how much media each user may upload.
    With a quota set at 25 MiB for example, users may be able to share about
    200 photos (depending on the original quality and how it got compressed
    at upload time).
  * Optional support for 'Explicit' (NSFW) media. If enabled, Explicit content
    is hidden except to users who opt-in to see it on their settings.
* Optional **Webcam Chat Room** integration
  * [BareRTC](https://git.kirsle.net/apps/BareRTC) is a Free & Open Source
    multi-user chat room application with video streaming (webcam) support
    included.
  * BareRTC was created specifically for the upstream website that GoSocial
    was branched from. As such, GoSocial has tight integration support with
    BareRTC 'out of the box.'
  * User Authentication (profile pictures, admin status), Profile Cards,
    Block Lists, Friendship Status, the ability to Report chat messages to
    your site's admin, etc. are all included.
* **User Safety**
  * Block lists: when users block each other, the site will completely hide
    the pair of users from each other everywhere, giving an identical
    appearance that the other user may have just deleted their account entirely.
  * Privacy Settings: users can limit who can send them messages, view
    their profile page, or comment on their content.
  * Two-Factor Auth and Login Sessions for your users to secure their own
    accounts.
* **Admin**
  * An 'Admin Flag' for user accounts and 'Admin Permission Scopes' to control
    which specific admin features that user has access to. An admin who has no
    scopes assigned has very limited permission to actually do anything beyond
    the normal user capabilities.
  * Feedback & Reports: there are handy Report buttons all throughout the site
    to enable users to easily flag inappropriate content for admin review.
  * Change Logs keep a history of when things on the site were modified, such as
    a history of friend requests, photo gallery changes, etc.
  * Admin dashboards to get insights about a user account (see their block lists,
    media usage quota, IP address history, etc.)
* **Security**
  * Protection from CSRF (Cross Site Request Forgery) on every POST-able form.
  * Rate limiting on failed login attempts and to reduce spam.
  * Optional Cloudflare Turnstile CAPTCHA support to protect your signup and
    public contact forms from spammers.
  * Optional [Signed Photo URLs](#signed-photo-urls-nginx) to protect the static
    photos shared by your users (so the direct link to the .jpg image has a URL
    signature unique to the logged-in viewer to protect against off-site deep
    links to the content).

The **Paid & Licensed Editions** come with additional features that were built
for the upstream production website that this codebase was branched from. Get
in touch to inquire about access to these features and to discuss what your
project's needs are and pricing.

* **Certification Photos**
  * Create a barrier to entry that helps keep fake profiles and spam bots off
    your platform. All social features of the site (chat, forums, etc.) can be
    gated to 'Certified Users Only' to protect your community from bad actors.
  * This workflow requires new user accounts to submit a selfie of themselves
    holding onto a sheet of paper that has their username, the site's name, and
    today's date written on it.
  * Admins review submitted Certification Photos and can approve them or reject
    them with an explanation sent back to the user.
* **Photo Gallery** Enhancements
  * On the Site Gallery, users may 'Mute' people who they don't want to see
    there and have the muted person's content not appear for them again.
  * If 'Explicit' media is enabled, your community members may help to flag
    photos that should have been marked as 'Explicit,' with a workflow for the
    photo's owner to appeal that decision and have an admin weigh in.
* **Video Gallery**
  * Allow your users to upload videos with audio, with an ffmpeg video encoding
    workflow.
  * Chunked file uploads: arbitrarily large videos may be uploaded and they are
    uploaded in multiple chunks, for robustness against network outages or
    failures. A failed video upload can be resumed later with only the still
    pending chunks needing to be uploaded.
  * Arbitrary max upload size: you can set a hard limit on the total size of the
    original media uploaded. After encoding, the final size on disk will often
    be smaller than the original upload.
  * An encoding worker uses ffmpeg to encode the upload to be suitable for Web
    playback (e.g. using h.264 and AAC for video/audio codecs, scaling down to
    a max resolution like 1080p, etc.)
* **Events**
  * A fully featured 'Events & RSVP' feature where users can schedule their own
    (in-person) events and manage RSVPs from others who want to attend the event.
  * Event Photo, Comment Form, Calendar Links.
* **Travel Plans**
  * Allow your users to advertise their upcoming travel plans in case they want
    to meet up with locals of that area. Start+End Dates, City, GPS location,
    custom description and comment threads.
* **Places**
  * Allow your community to 'crowd source' a database of interesting places in
    the world relating to your community's niche.
  * Users may write a description, upload a photo, mark a spot on the map where
    the place exists, checkbox all the features and amenities of the place,
    list directions and parking information, and each place has its own
    discussion forum as well.
  * Map View to browse all the places visually on a map of the world.
* **Miscellaneous**
  * Footprints: allow users to see who has visited their profile page.
  * Deleted User Memories: store a user's block lists when they delete their
    account, so if they sign up later with the same username or e-mail you can
    restore their blocked lists for them.
  * Optional: Admin Content Approval -- screen all user submitted media
    (photos/videos) and let your admin team check them out first to ensure
    appropriateness for your community.
* Optional **Specific Vendor Integrations**
  * The upstream website that GoSocial was branched from had integrations with
    specific business partners. In case your platform would want to use these
    same partners, the source code for these integrations can also be made available.
  * Yoti: Age Verification as part of the Certification Photos workflow.
  * CCBill: payment provider for paid supporter features.

## Tech Stack

On the technical side, some of the features of GoSocial's codebase include:

* Backend written in Go using largely the standard library (net/http, html/template).
* Database: [GORM](https://gorm.io/) for the ORM and query builder, with built-in
  support for PostgreSQL (preferred) or SQLite. Some website features require
  Postgres, notably the location-aware ones like Who's Nearby, which are not
  supported when using SQLite.
* Redis cache for rate limiting and temporary tokens (new account e-mail verification
  URLs, forgotten password links sent by e-mail, etc.)
* Front-end uses server-side rendered templates (html/template) and minimal
  JavaScript (mostly vanilla JS, Vue.js on some pages where appropriate).
  The default front-end design uses [Bulma.css](https://bulma.io/).

## Dependencies

You may need to run the following services along with this app:

* A **Redis cache** server: [redis.io](https://redis.io)
* (Optional) a **PostgreSQL database:** [postgresql.org](https://www.postgresql.org/)

The website can also run out of a local SQLite database which is convenient
for local development. The production server runs on PostgreSQL and the
web app is primarily designed for that.

For WebP image support, install the dependency (e.g. on Debian):

```bash
sudo apt -y install libwebp-dev
```

## Building the App

This app is written in Go: [go.dev](https://go.dev). You can probably
get it from your package manager, e.g.

* macOS: `brew install golang` with homebrew: [brew.sh](https://brew.sh)
* Linux: it's in your package manager, e.g. `apt install golang`

Use the Makefile (with GNU `make` or similar):

* `make setup`: install Go dependencies
* `make build`: builds the program to ./gosocial
* `make run`: run the app from Go sources in debug mode

Or read the Makefile to see what the underlying `go` commands are,
e.g. `go run cmd/gosocial/main.go web`

## Configuring

On first run it will generate a `settings.json` file in the current
working directory (which is intended to be the root of the git clone,
with the ./web folder). Edit it to configure mail settings or choose
a database.

For simple local development, just set `"UseSQLite": true` and the
app will run with a SQLite database.

### Hard-coded Site Configuration

The `pkg/config/` directory contains hard-coded config settings for
your website. Take a look through these files and customize anything
that you need.

These settings are mainly the 'static / non-variable' controls for your
site -- things like the site's Title and Branding, as well as available
enum options for user profile fields (select box options), the page
size options for various parts of the site, etc., which don't typically
need to be dynamically configurable.

The `settings.json` file by contrast is only for _variable_ configuration
(such as database credentials, API keys, etc.) which should not be
committed as part of the codebase or hard-set at compile time of the
app.

### Postgres is Highly Recommended

This website is intended to run under PostgreSQL and some of its
features leverage Postgres specific extensions. For quick local
development, SQLite will work fine but some website features will
be disabled and error messages given. These include:

* Location features such as "Who's Nearby" (PostGIS extension)
* "Newest" tab on the forums: to deduplicate comments by most recent
  thread depends on Postgres, SQLite will always show all latest
  comments without deduplication.

### PostGIS Extension for PostgreSQL

For the "Who's Nearby" feature to work you will need a PostgreSQL
database with the PostGIS geospatial extension installed. Usually
it might be a matter of `dnf install postgis` and activating the
extension on your gosocial database as your superuser (postgres):

```psql
create extension postgis;
```

If you get errors like "Type geography not found" from Postgres when
running distance based searches, this is the likely culprit and the above
command will create the needed extension in your database.

### Signed Photo URLs (NGINX)

The website supports "signed photo" URLs that can help protect the direct
links to user photos (their /static/photos/*.jpg paths) to ensure only
logged-in and authorized users are able to access those links.

This feature is not enabled (enforcing) by default, as it relies on
cooperation with the NGINX reverse proxy server
(module ngx_http_auth_request).

In your NGINX config, set your /static/ path to leverage NGINX auth_request
like so:

```nginx
server {
    # your boilerplate server info (SSL, etc.) - not relevant to this example.
    listen 80 default_server;
    listen [::]:80 default_server;

    # Relevant: setting the /static/ URL on NGINX to be an alias to your local
    # gosocial static folder on disk. In this example, the git clone for the
    # website was at /home/www-user/git/gosocial/website, so that ./web/static/
    # is the local path where static files (e.g., photos) are uploaded.
    location /static/ {
        # Important: auth_request tells NGINX to do subrequest authentication
        # on requests into the /static/ URI of your website.
        auth_request /static-auth;

        # standard NGINX alias commands.
        alias /home/www-user/git/gosocial/website/web/static/;
        autoindex off;
    }

    # Configure the internal subrequest auth path.
    # Note: the path "/static-auth" can be anything you want.
    location = /static-auth {
        internal;  # this is an internal route for NGINX only, not public

        # Proxy to the /v1/auth/static URL on the web app.
        # This line assumes the website runs on localhost:8080.
        proxy_pass http://localhost:8080/v1/auth/static;
        proxy_pass_request_body off;
        proxy_set_header Content-Length "";

        # Important: the X-Original-URI header tells the web app what the
        # original path (e.g. /static/photos/*) was, so the web app knows
        # which sub-URL to enforce authentication on.
        proxy_set_header X-Original-URI $request_uri;
    }
}
```

When your NGINX config is set up like the above, you can edit the
settings.json to mark SignedPhoto/Enabled=true, and restart the
website. Be sure to test it!

On a photo gallery view, all image URLs under /static/photos/ should
come with a ?jwt= parameter, and the image should load for the current
user. The JWT token is valid for 30 seconds after which the direct link
to the image should expire and give a 403 Forbidden response.

When this feature is NOT enabled/not enforcing: the jwt= parameter is
still generated on photo URLs but is not enforced by the web app.

## Usage

The `gosocial` binary has sub-commands to either run the web server
or perform maintenance tasks such as creating admin user accounts.

Run `gosocial --help` for its documentation.

Run `gosocial web` to start the web server.

```bash
gosocial web --host 0.0.0.0 --port 8080 --debug
```

## Create Admin User Accounts

Use the `gosocial user add` command like so:

```bash
$ gosocial user add --admin \
  --email name@domain.com \
  --password secret \
  --username admin
```

Shorthand options `-e`, `-p` and `-u` can work in place of the longer
options `--email`, `--password` and `--username` respectively.

After the first admin user is created, you may promote other users thru
the web app by using the admin controls on their profile page.

## Docker-Compose

While this app is easy to deploy manually, there is Dockerfile support for
those who prefer to go that route. This section will contain some advice on
how to manage the app with Docker Compose.

```bash
docker-compose up
```

Take a look inside the docker-compose.yml.

It has some bind mounts to provide the settings.json file from your host for
configuring the app.

Tip: use `docker ps` to see the running images, and note their Names if you
want to manually log into them.

#### Migrating an existing Postgres DB

After initializing the docker-compose cluster, you can import a pgdump.sql
to populate the database like:

```bash
# The NAME from `docker ps` is website_postgres_1 in this example.
sudo docker exec -i website_postgres_1 psql -d gosocial -U gosocial < pgdump.sql
```

Note: if we used the default `postgres` user the command might look like:

```bash
docker exec -i my-postgres-container psql -U postgres < pgdump.sql
```

#### Run maintenance commands on gosocial

You can run the gosocial binary from the web container like:

```bash
sudo docker exec -i website_web_1 ./gosocial help
```

e.g. to create the first admin user:

```bash
sudo docker exec -i website_web_1 ./gosocial user add --admin \
    --u admin -e root@localhost -p admin
```

To import a worldcities.csv database for the city search box (the `-`
argument for `gosocial setup locations --input` will read from STDIN):

```bash
sudo docker exec -i website_web_1 ./gosocial setup locations -i - < worldcities.csv
```

## A Brief Tour of the Code

* `cmd/gosocial/main.go`: the entry point for the Go program.
* `pkg/webserver.go`: the entry point for the web server.
* `pkg/config`: mostly hard-coded configuration values - all of the page
  sizes and business logic controls are in here, set at compile time. For
  ease of local development you may want to toggle SkipEmailValidation in
  here - the signup form will then directly allow full signup with a user
  and password.
* `pkg/controller`: the various web endpoint controllers are here,
  categorized into subpackages (account, forum, inbox, photo, etc.)
* `pkg/log`: the logging to terminal functions.
* `pkg/mail`: functions for delivering HTML email messages.
* `pkg/markdown`: functions to render GitHub Flavored Markdown.
* `pkg/middleware`: HTTP middleware functions, for things such as:
    * Session cookies
    * Authentication (LoginRequired, AdminRequired)
    * CSRF protection
    * Logging HTTP requests
    * Panic recovery for unhandled server errors
* `pkg/models`: the SQL database models and query functions are here.
    * `pkg/models/deletion`: the code to fully scrub wipe data for
      user deletion (GDPR/CCPA compliance).
* `pkg/photo`: photo management functions: handle uploads, scale and
  crop, generate URLs and deletion.
* `pkg/ratelimit`: rate limiter for login attempts etc.
* `pkg/redis`: Redis cache functions - get/set JSON values for things like
  session cookie storage and temporary rate limits.
* `pkg/router`: the HTTP route URLs for the controllers are here.
* `pkg/session`: functions to read/write the user's session cookie
  (log in/out, get current user, flash messages)
* `pkg/templates`: functions to handle HTTP responses - render HTML
  templates, issue redirects, error pages, ...
* `pkg/utility`: miscellaneous useful functions for the app.

## Cron workers

You can schedule the `gosocial vacuum` command in your crontab. This command
will check and clean up the database for things such as: orphaned comment
photos (where somebody uploaded a photo to post on the forum, but then didn't
finish creating their post), expire off old notifications and chat room DMs,
clean up expired login sessions, etc.

```cron
0 2 * * *  cd /home/gosocial/git/website && ./gosocial vacuum
```

For Video Gallery support (not part of the Free & Open Source Edition), you
would want to run the video encode worker as a long-running service on your
web server. The worker will await new video uploads and then encode them with
ffmpeg.

```bash
./gosocial worker videos
```

## License

GPLv3.
