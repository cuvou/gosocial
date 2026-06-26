# GoSocial Setup Guide

This document was based on installation notes for the upstream website that
GoSocial's code was based on. It provides an in-depth guide to manually
deploying the app in a production environment on a fresh install of Debian 13
Trixie, but should be largely applicable to installing it on any other modern
Linux server.

* [Dependencies & Server Setup](#dependencies--server-setup)
* [Building GoSocial](#building-gosocial)
* [PostgreSQL Database](#postgresql-database)
* [NGINX](#nginx)
* [Set up GeoIP Database](#set-up-geoip-database)
* [World Cities Database](#world-cities-database)
* [Supervisor Configs](#supervisor-configs)
* [Create the First Admin Account](#create-the-first-admin-account)
* [Cron Jobs](#cron-jobs)
* [Videos Worker](#videos-worker)

# Dependencies & Server Setup

On Debian, the required and recommended dependencies to install are as follows:

* Required:
	* postgresql
	* postgis
	* redis-server
	* libwebp-dev
	* build-essential
	* ffmpeg
	* make
* Recommended:
	* nginx (reverse proxy server to terminate SSL and serve static files efficiently)
	* certbot (free SSL cert provider)
	* ufw (firewall)
	* net-tools
	* git
	* rsync
	* supervisor

On Debian, the full suite of software I installed on the production server were as follows:

```bash
sudo apt update
sudo apt install ufw net-tools nginx redis-server postgresql postgis \
	libwebp-dev git rsync certbot supervisor nodejs npm make \
	build-essential ffmpeg
```

## Go

Your package manager may provide a new-enough version of Go, but for my own taste, it's recommended to grab the latest version of Go from https://go.dev -- especially on a stable distribution like Debian, the packaged version of Go may be too old to compile this app.

```bash
# EXAMPLE: download Go 1.26.4
wget https://go.dev/dl/go1.26.4.linux-amd64.tar.gz

# Extract and install it to /opt/go
tar -xzvf go1.26.4.linux-amd64.tar.gz
sudo rsync -av ./go/ /opt/go/

# So that the Go binary is in the following location:
/opt/go/bin/go version
```

You can install Go anywhere else that you like, /opt/go is my favorite place.

Be sure to update your `~/.bash_aliases` or similar file for your login shell:

```bash
export PATH="$PATH:/opt/go/bin"
export GOPATH="$HOME/go"
```

This adds /opt/go/bin to your $PATH so that the `go` command works, and sets your `$GOPATH` to `~/go` so installed Go dependencies will be in your home folder.

# Building GoSocial

It's recommended to create a non-privileged user on your server that will run the app. We'll call the user `gosocial`:

```bash
# Create the user and log in as it.
sudo adduser gosocial
sudo su -l gosocial
```

Put the GoSocial source code on your server, e.g. by git download:

```bash
# As user gosocial
mkdir git && cd git/
git clone https://github.com/cuvou/gosocial
cd gosocial/
```

If you are also using the BareRTC webcam chat room server, download that to your server as well.

Building the apps:

```bash
###
# GoSocial
cd ~/git/gosocial
make build

###
# BareRTC
cd ~/git/BareRTC

# One-time: install JavaScript dependencies to build the front-end app.
npm install

# Build the BareRTC Go app.
make build

# Build the front-end JS app.
npm run build
```

In both GoSocial and BareRTC, running `make run` can start the Go apps locally in your terminal for testing and initial setup. For production use, you'll want to set them up as long-running services (see [Supervisor Configs](#supervisor-configs) below).

# PostgreSQL Database

Ref: https://wiki.debian.org/PostgreSql

Quick start to initialize a PostgreSQL database for gosocial:

```bash
kirsle@web:~$ sudo su -l postgres
postgres@web:~$ createuser --pwprompt gosocial
Enter password for new role:
Enter it again:
postgres@web:~$ createdb -O gosocial gosocial
postgres@web:~$ psql gosocial
psql (17.6 (Debian 17.6-0+deb13u1))
Type "help" for help.

gosocial=# create extension postgis;
CREATE EXTENSION
```

For BareRTC, you may create a SQL user with restricted access only to the direct_messages table:

```sql
CREATE ROLE barertc LOGIN PASSWORD 'barertc' VALID UNTIL 'infinity';
GRANT CONNECT ON DATABASE gosocial TO barertc;

GRANT SELECT, INSERT, UPDATE, DELETE ON public.direct_messages TO barertc;
REVOKE ALL ON SCHEMA public FROM barertc;
```

# NGINX

NGINX is a reverse proxy server that will sit in front of GoSocial and provide SSL termination, efficient static file access, optional signed photo URL support, and optionally server multiple hostnames so you can have GoSocial and BareRTC running together on the same machine.

As gosocial user: `chmod 755 /home/gosocial` so that NGINX has permission to serve your static files.

## SSL Setup

Generate OpenSSL dhparams:

```bash
sudo openssl dhparam -out /etc/nginx/dhparam.pem 4096
```

For my personal taste, I like to have my SSL settings centralized in /etc/nginx/ssl_params where I also create an alias for the /.well-known URI for the CertBot Let's Encrypt SSL certificate workflow:

```nginx
# /etc/nginx/ssl_params

# Common SSL security settings
ssl_session_timeout 5m;

ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-SHA384;
ssl_session_cache shared:SSL:10m;
ssl_dhparam /etc/nginx/dhparam.pem;

# So the Acme client can use the htdocs method
location /.well-known {
	alias /var/www/html/.well-known;
}
```

Get free SSL certificates with Let's Encrypt.

```bash
sudo apt install certbot

# If you already have NGINX running, use the --webroot method with the default
# root of /var/www/html. Note: in my above ssl_params, I set the /.well-known
# alias to point to /var/www/html/.well-known so this command will work again
# in the future after my GoSocial website has been configured as well.
sudo certbot certonly --webroot -w /var/www/html -d example.com -d www.example.com

# If you have NOT yet started NGINX, you can use the standalone method of
# CertBot where it listens on its own HTTP server.
sudo certbot certonly --standalone -d example.com -d www.example.com
```

Note: if you will be using the BareRTC chat server, add a `-d chat.example.com` domain to your cert (or create a distinct certbot cert for that separately).

## NGINX Site Config

On Debian, the /etc/nginx directory has 'sites-available' and 'sites-enabled' folders. This example assumes that layout is being used. On Fedora or other distros, refer to your documentation for advice on where to place your config files.

Here is a full example NGINX config from my production server:

```nginx
# /etc/nginx/sites-available/example.com

server {
	server_name www.example.com;
	listen 443 ssl http2;
	listen [::]:443 ssl http2;

	ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
	ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;
	include ssl_params;

	# GoSocial app
	location / {
		include uwsgi_params;

		# X-Real-IP header: if you have a Cloudflare proxy in front of your
		# server, this will use the Cf-Connecting-IP header from Cloudflare
		# which contains the end user's true IP address.
		#
		# For non-Cloudflare installs, set the X-Forwarded-For header so that
		# NGINX will attach the end user's IP address.
		#
		# In GoSocial's settings.toml, enable the 'UseXForwardedFor' setting
		# and GoSocial will use the X-Real-IP or X-Forwarded-For headers (in
		# that order) to resolve the end user's true IP address (useful for
		# GeoIP locations and logging).
		proxy_set_header X-Real-IP $http_cf_connecting_ip;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

		# Proxy this request to GoSocial on its default port 8080.
		proxy_pass http://127.0.0.1:8080;

		# Optional: set a max upload size for users submitting media
		# to your website.
		client_max_body_size 10M;
	}

	# Static file access. Allow NGINX to directly serve your static
	# files to take load off the Go app needing to do so.
	location /static/ {
		alias /home/gosocial/git/website/web/static/;
		autoindex off;

		# If using the Signed URLs feature, uncomment this line.
		#auth_request /static-auth;
	}

	# This block is for the Signed URLs feature to help protect the static
	# photos so only the logged-in viewer can load them.
	location /static-auth {
		internal;
		proxy_pass http://127.0.0.1:8080/v1/auth/static;
		proxy_pass_request_body off;
		proxy_set_header Content-Length "";
		proxy_set_header X-Original-URI $request_uri;
		proxy_set_header X-Real-IP $http_cf_connecting_ip;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
	}
}

# HTTPS handler for a 'wrong domain' for your site (optional).
# If you want your site to canonically be 'www.example.com' but the user
# instead visited just 'example.com' this block will redirect them to the
# canonical version.
server {
	server_name example.com;
	listen 443 ssl http2;
	listen [::]:443 ssl http2;

	ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
	ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;
	include ssl_params;

	return 301 https://www.example.com$request_uri;
}

# HTTP forwarding: if a user visits the non-SSL version, redirect them
# to your canonical HTTPS URL.
server {
	server_name example.com www.example.com;
	listen 80;
	listen [::]:80;
	return 301 https://www.example.com$request_uri;
}

# vim:ft=nginx
```

If you are also setting up the BareRTC chat server:

```nginx
# /etc/nginx/sites-available/chat.gosocial.com
server {
	server_name chat.example.com;
	listen 443 ssl http2;
	listen [::]:443 ssl http2;

	ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
	ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;
	include ssl_params;

	# Optional: use the GoSocial favicon.
	location /favicon.ico {
		alias /home/gosocial/git/gosocial/web/static/favicon.ico;
	}

	# BareRTC Go app.
	location / {
		include uwsgi_params;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_pass http://127.0.0.1:9000;

		# Important: WebSocket handling, make NGINX forward the necessary
		# headers to allow the client to connect to BareRTC.
		proxy_http_version 1.1;
		proxy_set_header Upgrade $http_upgrade;
		proxy_set_header Connection "Upgrade";
		proxy_set_header Host $host;
	}
}

# HTTP forward
server {
	server_name chat.example.com;
	listen 80;
	listen [::]:80;
	return 301 https://chat.example.com$request_uri;
}

# vim:ft=nginx
```

On Debian, symlink the sites-available folders into sites-enabled to activate their configs:

```bash
cd /etc/nginx/sites-enabled
sudo ln -s /etc/nginx/sites-available/example.com ./
sudo ln -s /etc/nginx/sites-available/chat.example.com ./
```

Enable or reload the NGINX config:

```bash
# If NGINX is not yet running or enabled.
sudo systemctl enable --now nginx
# -or-
sudo systemctl start nginx

# If NGINX was already running, reload its config.
sudo systemctl reload nginx
```

# Set up GeoIP Database

GoSocial uses the free MaxMind GeoIP database to map user IP addresses to locations on the Earth.

Log in to https://www.maxmind.com

Install GeoIP Update: https://dev.maxmind.com/geoip/updating-databases/

Edit /etc/GeoIP.conf with your MaxMind credentials:

```
AccountID 8*****
LicenseKey ioiaLS_*****
EditionIDs GeoLite2-Country GeoLite2-City
```

Run the `geoipupdate` command and set it up in cron to keep it updated:

```cron
48 6 * * 0,3 /usr/local/bin/geoipupdate
```

# World Cities Database

Some features of GoSocial utilize a World Cities Database provided by [SimpleMaps.com](https://simplemaps.com).

Download the free Basic World Cities Database from https://simplemaps.com/data/world-cities

To import it, use the subcommand from the GoSocial binary:

```bash
./gosocial setup locations -i worldcities.csv
```

# Supervisor Configs

This section is about how to configure GoSocial and BareRTC to launch automatically when your web server boots up. You can use any supervisor software you like (such as SystemD), but for my personal taste, I like to use the software called `supervisor` as it keeps my custom services _cleanly_ separated away from system-level services that came with my Linux distribution.

The `supervisor` program is fairly simple to use and if you prefer SystemD instead, the supervisor config examples below are very readable so you can adapt the commands/working directory info to your service manager of choice.

For GoSocial:

```ini
# /etc/supervisor/conf.d/gosocial.conf

[program:gosocial]
command = /home/gosocial/git/gosocial/gosocial web
directory = /home/gosocial/git/gosocial
user = gosocial
```

For BareRTC:

```ini
# /etc/supervisor/conf.d/barertc.conf

[program:barertc]
command = /home/gosocial/git/BareRTC/BareRTC -address 127.0.0.1:9000 2>&1
directory = /home/gosocial/git/BareRTC
user = gosocial
```

If you also use the chatbot that comes with BareRTC, here is an example how to run that as well:

```ini
# /etc/supervisor/conf.d/chatbot.conf

[program:chatbot]
command = /home/gosocial/git/BareRTC/BareBot run ./chatbot 2>&1
directory = /home/gosocial/git/BareRTC
user = gosocial
```

Enable supervisor and add/start GoSocial:

```bash
# One-time: enable Supervisor.
sudo systemctl enable supervisor

# Any time you update /etc/supervisor/conf.d files,
# have it re-read your config files:
sudo supervisorctl reread

# When you've added a new program to Supervisor:
sudo supervisorctl add gosocial

# Optional: adding BareRTC and BareBot
sudo supervisorctl add barertc
sudo supervisorctl add chatbot
```

# Create the First Admin Account

To initialize your first admin account, the easiest way is to use the gosocial program's subcommand:

```bash
./gosocial user add --admin \
	--username admin
	--email admin@example.com
	--password secret
```

Once the GoSocial app is up and running, log in as your admin account.

Recommended next steps:

1. Change your admin user's password on the Settings -> Account Settings page (/settings/account)
2. Initialize your admin permission groups on the Admin -> Admin Permissions Management page (/admin/scopes URI).

The first admin account that initializes the Admin Permission Groups will become the 'Super User' with all admin scopes granted to them, and they will therefore be able to promote other accounts to admin, assign them to groups and hand out permissions, etc.

# Cron Jobs

Here are some recommended cron configs.

As user gosocial (`crontab -e`):

```cron
# Run the GoSocial vacuum command once per day to clean up your database,
# expire orphaned forum photo attachments, etc.
0 2 * * *  cd /home/gosocial/git/website && ./gosocial vacuum

# Optional: export your Postgres DB for backup purposes.
0 2 * * *  pg_dump -F c -O > /home/gosocial/pgdump.sql.gz
```

As user root:

```cron
# Reload NGINX periodically, so that when your SSL certs are renewed by
# CertBot, NGINX will update and use the latest certs before the old
# ones expire and show errors to your users!
0 2 * * *  systemctl reload nginx

# Keep the MaxMind GeoIP database updated.
36 10 * * 0,3 /usr/bin/geoipupdate
```

# Videos Worker

The Free & Open Source Edition of GoSocial does not come with the Videos feature. However, if you have a licensed version of the codebase that provides this, these are the steps to set that up.

The video worker uses Docker to safely run the ffmpeg encode commands in a sandboxed environment.

Install Docker and allow your gosocial user to run the `docker` command without sudo:

```bash
sudo apt install docker
sudo usermod -aG docker gosocial
```

Configure the `gosocial worker videos` command to run in the background:

```ini
# /etc/supervisor/conf.d/videos.conf
[program:videos]
command = /home/gosocial/git/gosocial/gosocial worker videos 2>&1
directory = /home/gosocial/git/gosocial
user = gosocial
```

The video worker sits idle and waits for newly uploaded videos that need processing. It will encode only one video at a time, on a first come, first served basis. The Docker ffmpeg command is configured to only use one CPU core and to limit the CPU usage during encodes. See the `pkg/config.go` for details on the ffmpeg command and to customize it if you need.
