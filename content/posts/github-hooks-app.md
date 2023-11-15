+++
date = 2021-06-04T11:28:10+03:00
title = "Step by step guide for creating GitHub hooks App"
tags = ["GitHub", "webhooks"]
categories = ["DevOps"]
aliases = ["/posts/2021-06-04-github-hooks-app/"]
+++

GitHub has well-documented REST API endpoints and description for
OAuth2 apps, but the quick-start for creating new hooks app for
marketplace is not really clear: you need to read a lot of documentation
just to try it. I'm going to explain here how to create new GitHub app
in a few steps, and then you will be able to read the whole documentation
if interested.

First you need to understand if you really need a GitHub app.
You have three options to perform GitHub API calls:
 - Using personal access token
 - Creating OAuth application
 - Creating GitHub app

[This diagram](https://docs.github.com/en/developers/apps/getting-started-with-apps/about-apps#determining-which-integration-to-build)
may help you to decide what kind of application is more suitable.
Also, GitHub explains
[the difference](https://docs.github.com/en/developers/apps/getting-started-with-apps/differences-between-github-apps-and-oauth-apps)
between OAuth and Apps quite verbosely.

## Requirements

To create a new GitHub app you will need:
 - Web server with DNS name and HTTPS to serve web-hooks processor:
 GitHub will send HTTP requests for configured events to this URL.
 - GitHub account to create new application

## Configuration

The steps for creating a new GitHub application:
 - Go to you GitHub profile, settings, DeveloperSettings, GitHub Apps, click
 "New GitHub App" (or just follow [the link](https://github.com/settings/apps/new));
 fill required inputs in create-app form, pay attention to "Webhook URL", it should
 be you server URL + hooks path. Click create.
 - Create web-app for webhooks processing, deploy it to the server. It should
 handle correctly hooks URL specified in first-step as "Webhook URL"
 - Go to the down of app page and generate private key for this app, download it
 and save to the hooks server. Also, copy "App ID" and save it on server too.
 - Generate random string locally as webhooks secret,
 save it in app settings as "Webhook secret" and on server too.
 - Configure permissions and events on App page settings: permissions specify what
 GitHub API calls your application is allowed to perform, it could be configured;
 events specify what webhooks App receives
 when installed. Both are configured on "Permissions & events" tab on GitHub App page.
 - On the App page, "General" tab: copy "App ID" to your web server.

## Hooks processing

Process hooks:
 - Using App private key and App ID create transport for GitHub API calls,
 it could be done once on server start. For Go you can use this library:
 [bradleyfalzon/ghinstallation](https://github.com/bradleyfalzon/ghinstallation).
 For other languages you can user [this approach](https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-a-github-app)
 or find another library. App authentication allows the client to perform
 high-level management information via API and retrieve access token for installation.
 - On each webhook received: validate webhook payload using webhooks secret
  (which you generated locally and saved as "Webhook secret" input on App page),
  parse payload, extract app installation ID from hook event
  (json path `.installation.id`), and using this installation ID gen new access token
  using App authentication. With this access token you can access GitHub API
  with permissions you've specified in "Permissions & events".

## Debugging

To check webhook events the GitHub sends to your App and view the responses
to GitHub, you can check App's page "Advanced" and "Recent Deliveries" section:
there GitHub displays all event payloads and HTTP requests, and the status of
delivery with hook's server responses.
