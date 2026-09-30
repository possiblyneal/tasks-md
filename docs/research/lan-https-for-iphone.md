# HTTPS an iPhone trusts, on the LAN

Researched 2026-09-30 for issue #110, part of map #92. It answers how
`todo api` can serve the browser client over HTTPS on home Wi-Fi so that an
iPhone shows no warning, opens the page standalone from the Home Screen, and
registers a service worker. For each way it records the one-time setup, what
renewal takes, what the way needs (a public domain, a DNS provider's API, a
profile on the phone), and whether it survives Tailscale being added later.

Each fact is tagged:

- **[docs]**: verified from a primary source, linked in the section or in the
  list below.
- **[inferred]**: reasoned from the docs, from this network's live config, or
  from a community report, and not stated by the owner of the behavior.
- **[untested]**: plausible and unverified. It needs a real iPhone to settle.

Sources:

- Apple, TLS rules since iOS 13: <https://support.apple.com/en-us/103769>
- Apple, 398-day limit and who it applies to: <https://support.apple.com/en-us/102028>
- Apple, trusting a manually installed root: <https://support.apple.com/en-us/102390>
- Apple, stricter TLS for system connections in iOS 27: <https://support.apple.com/en-us/126655>
- Apple, iCloud Private Relay: <https://support.apple.com/en-us/102602>
- MDN, secure contexts: <https://developer.mozilla.org/en-US/docs/Web/Security/Secure_Contexts>
- Let's Encrypt challenge types: <https://letsencrypt.org/docs/challenge-types/>
- Let's Encrypt, 6-day and IP certificates GA (2026-01-15): <https://letsencrypt.org/2026/01/15/6day-and-ip-general-availability.html>
- Let's Encrypt, IP certificates in Certbot (2026-03-11): <https://letsencrypt.org/2026/03/11/shorter-certs-certbot.html>
- Let's Encrypt, 90 to 45 days: <https://letsencrypt.org/2025/12/02/from-90-to-45.html>
- Let's Encrypt, DNS-PERSIST-01: <https://letsencrypt.org/2026/02/18/dns-persist-01.html>, and its deployment status thread: <https://community.letsencrypt.org/t/dns-persist-01-deployment-status-and-timeline/246468/17>
- CertMagic README: <https://github.com/caddyserver/certmagic>
- `golang.org/x/crypto/acme/autocert` source: <https://github.com/golang/crypto/blob/master/acme/autocert/autocert.go>
- mkcert source and README: <https://github.com/FiloSottile/mkcert> (`cert.go`, `README.md`)
- Tailscale, Enabling HTTPS: <https://tailscale.com/kb/1153/enabling-https>
- Tailscale, Serve: <https://tailscale.com/kb/1312/serve>
- Tailscale, Caddy certificates (`TS_PERMIT_CERT_UID`): <https://tailscale.com/kb/1190/caddy-certificates>
- Tailscale, DNS: <https://tailscale.com/docs/reference/dns-in-tailscale>
- This network: the router's `uci show dhcp`, `~/code/homelab/apps/traefik/CLAUDE.md`,
  `~/code/homelab/docs/networking/vlan-map.md`, and the live certificates on
  `server`, all read on 2026-09-30

---

## 0. The operator's situation

Read-only checks on 2026-09-30. **[inferred]** throughout, from live state.

- `todo api` runs on `dev` (`10.10.10.50`) as the `systemd --user` unit, on
  plain HTTP at `:8080`. It has been up since 2026-09-24.
- The iPhone has a static lease at `10.10.10.30` on the same `main` VLAN and
  Wi-Fi as `dev`. No VLAN boundary sits between them.
- **Tailscale is not installed** on `dev` (`tailscale: command not found`) or on
  the router.
- **No public domain** turns up in this repo or in `~/code/homelab`. Every name
  on this network is under `.lan`.
- The router is **OpenWrt 24.10.2** running dnsmasq, with
  `rebind_protection='0'`. A public name answering with a private address is
  passed through: `10.10.10.50.nip.io` and `10-10-10-50.sslip.io` both resolve
  to `10.10.10.50` through it. The router also holds a local record per `.lan`
  name (`list address '/<name>.lan/<ip>'`), so split-horizon DNS is already how
  this network works.
- **A private CA already exists and already fronts `dev` services.** `server`
  (`10.10.10.10`) runs Traefik with a leaf signed by `Homelab CA`:
  - The CA is RSA 4096 with `CA:TRUE` and `keyUsage` set to certSign and cRLSign.
    It is valid 2026-02-13 to 2036-02-11 and its CN is `Homelab CA`.
  - The leaf is RSA 2048 with SHA-256 and EKU `serverAuth`. It carries an
    explicit SAN per `.lan` name, and it is valid 2026-09-19 to 2028-12-22,
    which is exactly 825 days.
  - Traefik already routes three `dev` backends (`omniroute.lan`,
    `omniroute-redis.lan`, `agentmemory.lan`) to `10.10.10.50`. Adding a name
    takes three edits: a `dynamic.yml` router, a SAN in `lan-openssl.cnf` plus
    a leaf regeneration, and a router DNS record.
  - Nothing read here says whether the iPhone has `Homelab CA` installed and
    fully trusted. `linkding.lan`'s doc says "the browser must trust `ca.crt`".
    **[untested]**: check Settings > General > About > Certificate Trust
    Settings on the phone.

---

## 1. What the phone requires

### Secure context

- A service worker needs a secure context. `http://localhost`, `127.0.0.1` and
  `*.localhost` count as potentially trustworthy without TLS. **A LAN address
  over `http://` does not.** **[docs]** (MDN)
- So plain `http://10.10.10.50:8080` can go on the Home Screen, but it cannot
  register a service worker, which rules out the offline board #109 needs.
  **[inferred]**
- Tapping past a certificate warning in Safari is a per-visit exception. It
  does not make the origin trusted, and browsers refuse to register a service
  worker on an origin with a certificate error. **[inferred]** A self-signed
  leaf that no trusted root vouches for is therefore **not viable**.

### What iOS checks on a TLS server certificate

These rules apply to **every** TLS server certificate, including one chaining
to a root the person installed. **[docs]** (103769)

- RSA keys at least 2048 bits, for the leaf and for the issuing CAs.
- A SHA-2 signature. SHA-1 is not trusted.
- The DNS name must be in the **Subject Alternative Name**. A name that appears
  only in the CN is not trusted.
- For certificates issued after 2019-07-01: an EKU containing
  **`id-kp-serverAuth`**, and **825 days or fewer** of validity.
- The **398-day** limit (for certificates issued on or after 2020-09-01) applies
  **only to certificates from the roots preinstalled** with the OS. "This change
  will not affect certificates issued from user-added or administrator-added
  Root CAs." **[docs]** (102028)
- Validity is measured inclusive of `notBefore` and `notAfter`, and a day is
  86,400 seconds. Anything longer counts as another day. **[docs]** (102028)
  The Homelab leaf was minted with `-days 825`, so it sits exactly on the
  limit. Whether iOS counts it as 825 days or as 825 days plus one second is
  **[untested]**. mkcert avoids the question by issuing for 2 years and 3
  months, "which is always less than 825 days". **[docs]** (mkcert `cert.go`)
- iOS 26 and later negotiate TLS 1.3 and fail against servers that disable it.
  **[inferred]** (F5 K000160894, a vendor note) Go's `crypto/tls` and Traefik
  both enable TLS 1.3 by default. **[inferred]**
- In iOS 27 Apple tightens TLS for *system* connections: MDM, profile
  installation, app installation and software updates. Safari browsing is not
  on that list. **[docs]** (126655)

### iCloud Private Relay and split DNS

- A community report says that with Private Relay on, browsers on macOS ignore
  the local resolver's split-horizon answer for a **public** name and use the
  public one. **[inferred]** (desantolo.com, 2025-08) Private Relay can be
  turned off per Wi-Fi network. **[docs]** (102602)
- This matters to way B when the public DNS answer differs from the local one.
  A public A record that itself points to `10.10.10.50` gives both resolvers
  the same answer. Whether Private Relay then connects to a private address
  directly is **[untested]**.

---

## 2. Way A: a private CA whose root the iPhone trusts

### How it works

A CA the operator owns signs a leaf for a local name. The CA's root is
installed on the iPhone as a profile and then given full trust. **[docs]**
(102390; mkcert README)

- **Install on iOS:** open the root file (AirDrop, email, or an HTTP download)
  to get "Profile Downloaded". Install it under Settings > General > VPN &
  Device Management. Then turn it on under **Settings > General > About >
  Certificate Trust Settings > Enable full trust for root certificates**. A
  manually installed profile "isn't automatically trusted for SSL". **[docs]**
  (102390)
- **No MDM is required.** Apple *recommends* Configurator or MDM, which trust
  the root automatically, but a downloaded profile plus the toggle works on its
  own. **[docs]** (102390)
- The root needs a CN to appear in the Certificate Trust Settings list.
  **[docs]** (mkcert `cert.go` comment) `Homelab CA` has one. **[inferred]**

### Three shapes on this network

1. **Front `todo api` with the existing Traefik.** Add `todo.lan` to
   `dynamic.yml` pointing at `http://10.10.10.50:8080`, add a SAN and
   regenerate the leaf, and add the router record. `todo api` stays on plain
   HTTP and gains no code. **[inferred]** The costs:
   - `server` becomes a dependency of the board. The homelab's Orca doc already
     calls that trade a single point of failure and declined it for Orca.
   - The hop from `server` to `dev` stays plaintext, which is fine inside ADR
     0003's "the network is the boundary".
   - Every other `.lan` name already rides this leaf, so the phone trusts all of
     them together or none. **[inferred]**
2. **`todo api` terminates TLS itself** with a leaf signed by `Homelab CA`,
   under a new `-cert`/`-key` pair or a Go `tls.Config`. It needs the same SAN
   and DNS record and no Traefik route. **[inferred]**
3. **A fresh mkcert CA.** `mkcert -install` plus `mkcert todo.lan` gives an RSA
   2048 serverAuth leaf valid for 2 years 3 months. The root is `rootCA.pem`
   under `mkcert -CAROOT`. **[docs]** (mkcert) It adds a second root to the
   phone when one may already exist. **[inferred]**

A community tool does exactly this for PWA installation on iOS. It mints an
mkcert leaf for a LAN address, serves the root for the phone to install and
trust, and then installs the PWA over HTTPS. **[inferred]** (hunterirving.com
`mixapps/https_serve.py`) No WebKit bug turned up that blocks service workers
or standalone mode on a root the person trusted. A trusted chain is by
definition not a certificate error. **[inferred]** Confirming it on this phone
is **[untested]**.

### Setup, renewal, requirements

- **One-time:** install and trust the root on the phone (two screens), add the
  name to DNS and the SAN, and serve the leaf.
- **Renewal:** the leaf lasts at most 825 days, so a regeneration every two
  years or so. It is manual as the homelab runs it today and could be a
  timer. The root lasts 10 years (`Homelab CA` expires 2036), and the phone
  needs no action until the root changes. **[inferred]**
- **Public domain:** no. **DNS provider API:** no. **Profile on the phone:**
  yes, one CA profile with full trust. No MDM.
- **Internet dependency:** none. It works with the WAN down. **[inferred]**

### With Tailscale later

- The iPhone keeps trusting the root whatever network it is on. What changes
  is how `todo.lan` resolves off-LAN. **[inferred]**
- Tailscale can send a domain to a chosen nameserver ("restricted
  nameservers", that is, split DNS). **[docs]** (DNS in Tailscale) With
  `lan` pointed at `10.10.10.1` and a subnet router advertising
  `10.10.10.0/24`, the same name and the same certificate work over the
  tailnet. **[inferred]**
- Alternatively, add the host's `ts.net` or `100.x` name as another SAN.
  **[inferred]**
- Either way it survives; it needs a subnet router or an extra SAN.

---

## 3. Way B: a public domain's certificate through ACME DNS-01

### How it works

- Buy a domain, for example `example.net`. The ACME client proves control by
  writing a TXT record at `_acme-challenge.<name>`. The CA never has to reach
  the host, so the name can resolve to `10.10.10.50`. **[docs]** (Let's
  Encrypt challenge types)
- It "only makes sense ... if your DNS provider has an API". An
  `_acme-challenge` CNAME or NS delegation can hand the challenge to another
  zone, such as an acme-dns server, when the registrar has no API. **[docs]**
- The certificate is publicly trusted. There is **no profile on the phone**, and
  every device and browser trusts it. **[docs]** The name enters the public
  Certificate Transparency logs. **[docs]** (Tailscale's page states this for
  any Let's Encrypt name)

### Go-native options for `todo api`

- **CertMagic** gives full ACME with ARI. Setting `DNS01Solver` enables DNS-01
  through any `libdns` provider. It obtains, renews and serves inside the
  process. **[docs]** (CertMagic README)
- **`x/crypto/acme/autocert` cannot do this.** It supports only `tls-alpn-01`
  and `http-01`, and both need the CA to reach the host. **[docs]**
  (`autocert.go`) The lower-level `x/crypto/acme` package speaks the protocol,
  but DNS writing and renewal would be the operator's own code. **[inferred]**
- **lego** as a library or CLI, or **Caddy** or Traefik as a front, also do
  DNS-01. **[inferred]** Traefik is already on `server`, and its ACME resolver
  would replace the private leaf for that name.

### Where the name resolves

- **Public A record to `10.10.10.50`.** This works on this router because
  `rebind_protection` is off. **[inferred]** (live dig) Many stock routers and
  some resolvers drop private answers to public names, but that is not this
  network's case today.
- **Split horizon:** a router record `address=/todo.example.net/10.10.10.50`,
  the same mechanism the `.lan` names use. It may be bypassed by Private Relay
  (section 1). **[inferred]**

### Setup, renewal, requirements

- **One-time:** register a domain, choose a DNS host with an API or delegate
  `_acme-challenge`, store an API token on `dev`, and wire in CertMagic or a
  front.
- **Renewal:** automatic, and it must be. Let's Encrypt lifetimes shrink from
  90 days to 64 days on 2027-02-10 and to 45 days on 2028-02-16, with the
  authorization reuse window falling to 7 hours. "Manually renewing
  certificates is not recommended". **[docs]** (90-to-45 post) Renewal needs
  internet access and a working DNS API token. **[inferred]**
- **DNS-PERSIST-01** would let a single static TXT record authorize every
  renewal with no DNS API, but as of 2026-06-25 Let's Encrypt "will not be
  deploying dns-persist-01" until an open draft issue is resolved. **[docs]**
  Its status at 2026-09-30 is **[untested]**.
- **Public domain:** yes, about a domain's yearly fee. **DNS provider API:** yes,
  or a delegated challenge zone. **Profile on the phone:** no.

### With Tailscale later

- A public A record to `10.10.10.50` resolves anywhere. Over Tailscale it is
  reachable once a subnet router advertises the LAN. **[inferred]**
- Alternatively, a public record to the host's `100.x` address.
  Tailscale documents public DNS records for tailnet addresses as "relatively
  harmless". **[docs]** (DNS in Tailscale)
- The certificate itself needs no change. **[inferred]**

---

## 4. Way C: `tailscale cert`

### How it works

- Enable MagicDNS and HTTPS Certificates in the admin console. Then
  `tailscale cert` gets a Let's Encrypt certificate for
  `<machine>.<tailnet>.ts.net`. Tailscale writes the DNS-01 TXT record under
  `ts.net`, and the keys stay on the machine. **[docs]** (1153)
- Machine names and the tailnet name go into the public CT logs. **[docs]**
- **Renewal:** these are 90-day certificates, and 64 and then 45 as Let's
  Encrypt shortens them. **[inferred]** A certificate written to files by
  `tailscale cert` is **the operator's to renew**, and `tailscaled` does not
  renew it. Certificates held through the Caddy integration renew
  automatically. **[docs]** (1153) `tailscale serve` provisions certificates
  itself. **[docs]** (1312)
- **Go-native:** `tailscale.com/client/local.Client.GetCertificate` is a
  `tls.Config.GetCertificate` callback that fetches and renews through the
  local `tailscaled`. **[docs]** (1153) A non-root process such as this user
  unit needs `TS_PERMIT_CERT_UID=<user>` in `tailscaled`'s environment.
  **[docs]** (1190)
- **Public domain:** no, since `ts.net` is Tailscale's. **DNS provider API:**
  no. **Profile on the phone:** no, because the certificate is publicly
  trusted. It **does need Tailscale installed and logged in on `dev`**, and in
  practice on the phone as well.

### The LAN-only phone

- The `ts.net` name resolves through MagicDNS (`100.100.100.100`) to the host's
  `100.x` address. A random `*.tail0000.ts.net` name returns NXDOMAIN from
  `1.1.1.1`, and Tailscale's docs describe tailnet names as reachable through
  MagicDNS. **[inferred]** So a phone on Wi-Fi with the Tailscale app off
  cannot resolve the name, and could not reach a `100.x` address anyway.
- **A workaround the docs don't state:** a router record mapping
  `dev.<tailnet>.ts.net` to `10.10.10.50`. The phone then reaches the LAN
  address under a name the certificate covers, with Tailscale off. With
  Tailscale on, MagicDNS answers with `100.x` and the same certificate still
  matches. **[inferred]** **[untested]**
- Today this way needs a Tailscale account and daemon that v1.0 does not plan
  to have. It is the way that **needs** Tailscale rather than merely surviving
  it.

---

## 5. Other ways, briefly

- **Caddy's internal CA** (`tls internal`) is way A with Caddy as the CA and the
  front. The phone still needs the root installed and trusted. **[inferred]**
  It is not better than the `Homelab CA` that already exists.
- **Let's Encrypt IP-address certificates** have been GA since 2026-01-15. They
  must use the 6-day `shortlived` profile. **[docs]** They validate by
  `http-01` or `tls-alpn-01` only, because DNS-01 "cannot be used to validate
  IP Addresses". **[docs]** The CA must therefore reach the address from the
  internet, and `10.10.10.50` is private. **Not viable** for a LAN address.
  **[inferred]** (Publicly trusted certificates for private IP ranges are
  also barred by the Baseline Requirements. **[inferred]**)
- **nip.io / sslip.io** give DNS only, turning `10-10-10-50.sslip.io` into
  `10.10.10.50`, which works on this router. They give no certificate, and one
  for such a name needs `http-01` from the internet or DNS-01 on a zone the
  operator does not control. **Not viable** by themselves. **[inferred]**
- **Services that ship a wildcard certificate and its private key** (the
  traefik.me or local-ip.co pattern) publish the key. A published key is a
  key compromise the CA must revoke for, so trust can end without warning.
  **Not viable.** **[inferred]**
- **Self-signed with no trusted root**, or **plain `http://` on the LAN
  address**, are not a secure context (section 1). **Not viable.**
- **`.local` through mDNS** is a naming choice, not a certificate source. It
  works under way A (a private CA may sign `dev.local`). No public CA issues
  for it. **[inferred]**

---

## 6. Comparison

| | A: private CA (`Homelab CA`) | B: public domain, DNS-01 | C: `tailscale cert` |
| --- | --- | --- | --- |
| Phone shows no warning | Yes, after the root is trusted **[docs]** | Yes **[docs]** | Yes, once the name resolves **[docs]** |
| Standalone and service worker | Yes **[inferred]**, **[untested]** on this phone | Yes **[inferred]** | Yes **[inferred]** |
| One-time setup | Profile plus full-trust toggle on the phone. A SAN, a DNS record, and a Traefik route or a TLS listener | Buy a domain, DNS API token, CertMagic or a front, a DNS record | Tailscale on `dev` (and the phone), enable HTTPS, `TS_PERMIT_CERT_UID`, `GetCertificate` |
| Renewal | Leaf every ≤825 days, manual or a timer; root 2036 | Automatic, every 64 then 45 days, needs internet and API | Automatic through `GetCertificate`, serve or Caddy; manual if file-based |
| Public domain | No | Yes | No (`ts.net`) |
| DNS provider API | No | Yes, or a delegated zone | No |
| Profile on the phone | Yes, a CA profile, no MDM | No | No |
| Works with the WAN down | Yes | Until the certificate lapses | Until the certificate lapses |
| Survives Tailscale later | Yes, with split DNS and a subnet router, or an extra SAN | Yes, with a subnet router or an A record to `100.x` | Designed for it |
| Needs Tailscale in v1.0 | No | No | Yes |

---

## 7. Gaps

- Whether this iPhone already trusts `Homelab CA`. **[untested]**
- Whether iOS accepts the current `-days 825` Homelab leaf, which sits exactly
  on the inclusive 825-day limit. **[untested]** Regenerating it with
  `-days 824` removes the doubt without testing it.
- A service worker registering, and the offline board loading, in a standalone
  Home Screen app under a user-trusted root, on the iOS version the phone runs.
  **[untested]**
- iCloud Private Relay's effect on a `.lan` name, or on a public name answering
  with a private address, in Safari on iOS. **[untested]**
- DNS-PERSIST-01's production status at the time of reading. **[untested]**
- The router override for a `ts.net` name in way C. **[untested]**
