# Using NovelCheck away from home (Cloudflare Tunnel)

A Cloudflare Tunnel gives NovelCheck a secure web address, such as `https://books.yourname.com`, that works from anywhere: your phone on cellular, a friend's house, work. You don't open any ports on your router, and your home IP address stays hidden. Cloudflare's tunnel service is free.

NovelCheck has the tunnel connector built in, so there's nothing extra to install on TrueNAS.

## What you need

- A free **Cloudflare account** (dash.cloudflare.com).
- A **domain name** whose DNS is managed by Cloudflare, for example `yourname.com`. If you don't have one, you can buy one inside Cloudflare (**Domain Registration → Register Domains**), usually about $10 a year. If you already own one elsewhere, add it to Cloudflare with **Add a domain** and follow its steps to change the nameservers.
- About 10 minutes.

## Step 1: Create the tunnel in Cloudflare

1. Sign in at **dash.cloudflare.com** and open **Zero Trust** from the left menu. The first time, Cloudflare asks you to pick a team name and the **Free** plan. It may ask for a card, but the free plan isn't charged.
2. Go to **Networks → Tunnels** and click **Create a tunnel**.
3. Choose **Cloudflared** as the connector type, name it `novelcheck`, and click **Save tunnel**.
4. On the "Install and run a connector" page, Cloudflare shows install commands. **Don't run them.** Just copy one of them: they all contain your tunnel token, the long text starting with `eyJ`. You can paste the whole command into NovelCheck and it will pick out the token.
5. Click **Next**.

## Step 2: Choose your web address

On the next page (called **Public Hostnames**, or **Published application routes** in newer versions of the dashboard), add a route:

1. **Subdomain:** `books`, or anything you like.
2. **Domain:** pick your domain from the list.
3. **Service type:** `HTTP`.
4. **URL:** `localhost:8080`
5. Click **Save** (or **Complete setup**).

Your NovelCheck address is now the subdomain plus the domain, for example `books.yourname.com`.

`localhost:8080` is correct even though you normally open NovelCheck on port 30080: the connector runs inside NovelCheck itself.

## Step 3: Turn it on in NovelCheck

1. In NovelCheck, open **Admin → Delivery & Services → Remote access**.
2. Paste the token (or the whole command you copied) into **Tunnel token**.
3. Type your address, for example `books.yourname.com`, into **Public address**.
4. Tick **Turn on remote access** and click **Save & connect**.
5. Within about 10 seconds the status should say **Connected**. Open your address on your phone with Wi-Fi turned off to check.

## Step 4: Use it on your phones

Open the new `https://` address on each phone, sign in, and add it to the home screen (iPhone: **Share → Add to Home Screen**; Android: **Install app**). It works both at home and away, so everyone can use this one address.

The home-screen app from the new address is separate from any you made with the old `http://…:30080` address, so each person signs in once more.

Phone notifications need this `https://` address too. In the home-screen app, go to **Profile → Phone notifications** and tap **Turn on for this device**, then **Send a test**.

## Optional: an extra lock with Cloudflare Access

NovelCheck already needs a password, and it slows down repeated wrong guesses. For extra peace of mind, Cloudflare can ask for a one-time code sent by email before anyone even sees the NovelCheck page:

1. In Zero Trust, go to **Access → Applications → Add an application → Self-hosted**.
2. Enter your NovelCheck address, then add a policy with **Action: Allow** and **Include → Emails** listing your family's email addresses.
3. Save. Visitors now get an email code first, then the NovelCheck sign-in.

## Alternative: use TrueNAS's Cloudflared app instead

If you'd rather run the tunnel as its own TrueNAS app (for example because you already use it for other apps):

1. Do Step 1 above.
2. In TrueNAS, go to **Apps → Discover Apps**, install **Cloudflared**, and paste the token.
3. In Step 2, set the URL to `YOUR-TRUENAS-IP:30080` instead of `localhost:8080`.
4. Leave **Remote access** in NovelCheck turned off.

## Troubleshooting

- **Status stays "Starting" or shows "Retrying":** check the token. Copy it again from **Zero Trust → Networks → Tunnels → novelcheck → Configure**, paste it, and click **Save & connect**. The log box shows Cloudflare's messages.
- **"Cloudflare rejected the tunnel token":** the token was cut short or the tunnel was deleted. Copy a fresh one.
- **Connected, but the address shows a Cloudflare error page (502 or 1033):** the route's URL is wrong. It must be `localhost:8080` with type `HTTP`, or `YOUR-TRUENAS-IP:30080` if you use the separate Cloudflared app.
- **The address doesn't load at all:** first open **System → Check everything** and look at **Public address**. NovelCheck loads your address from the internet and tells you whether it really works.
  - **"Opens from the internet"** but your phone or computer says the page can't be found: that device's network remembers an old "not found" answer (for up to 30 minutes). Try your phone on mobile data with Wi-Fi off, wait a bit, run `ipconfig /flushdns` on Windows, or restart your router. A Pi-hole or ad-blocking DNS can also block new addresses.
  - **"doesn't exist in DNS yet"**: add the address under the tunnel's **Public Hostname** tab and check the domain is **Active** in the Cloudflare dashboard.
- **The log says "QUIC connection failed" and "suggested_protocol=http2":** harmless. Your network blocks UDP port 7844 to one of Cloudflare's regions, so cloudflared uses HTTP/2 instead. The "receive buffer size" warning is harmless too.
- **"cloudflared is not installed":** you're running an old NovelCheck image. Update NovelCheck (see "Updating NovelCheck" in the TrueNAS guide).
