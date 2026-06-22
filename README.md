<p align="center">
  <img src="ui/favicon.png" width="96" height="96" alt="Lighthouse Logo" />
</p>

<h1 align="center">Lighthouse</h1>

<p align="center">
  Hi! Welcome to <b>Lighthouse</b>. This is a simple tool to help you run and manage your web applications on a local server, like a Raspberry Pi or a spare computer, easily.
</p>

<p align="center">
  Lighthouse does the hard work of pulling your code from Git, building it, setting up local domains (like <code>myapp.local</code>), creating SSL certificates for HTTPS, and running your apps in the background.
</p>

---

## 1. How Lighthouse Works (The Architecture)

Here is a simple picture of how Lighthouse is built:

1. **The Web Dashboard (UI)**
   - Built with plain HTML, CSS, and a small JavaScript library called **Alpine.js**.
   - It is a single-page dashboard where you can see all your apps, click buttons to deploy them, start/stop them, and look at their logs.

2. **The Go Backend Server**
   - The main program is written in **Go**.
   - It uses a router library called **Chi** to handle web requests.
   - It saves all its information (like which apps are running and their ports) in a lightweight database called **SQLite**.

3. **How Apps Run in the Background (Systemd)**
   - Instead of using heavy Docker containers, Lighthouse uses a tool already inside Linux called **Systemd**.
   - Lighthouse creates a systemd service file (a `.service` file) for each app.
   - Linux then keeps the app running in the background and restarts it if it crashes.

4. **Making Domains Work (Dnsmasq)**
   - When you give an app a domain name (like `mycoolsite.local`), Lighthouse writes it into a tool called **Dnsmasq**.
   - Dnsmasq makes sure that when you type that domain in your web browser, it points directly to your local server.

5. **Security with HTTPS (SSL/TLS CA)**
   - Lighthouse acts as its own Certificate Authority (CA) using **OpenSSL**.
   - When you deploy a new app, Lighthouse automatically creates a security certificate for it.
   - This lets you open your local websites using `https://` without browser security warnings (after you trust the CA once!).

6. **Web Traffic Routing (Nginx)**
   - Lighthouse uses **Nginx** as a reverse proxy.
   - When a request comes to your server on port 80 (HTTP) or 443 (HTTPS), Nginx reads the domain name and sends the traffic to the correct port where your app is running.
   - All Nginx configs are version-controlled with **Git** in the `/etc/nginx/sites-available` directory so we can roll back configs if something goes wrong.

---

## 2. Installing and Running

### System Requirements
- Linux (Ubuntu, Debian, or Raspberry Pi OS)
- Nginx, Dnsmasq, and OpenSSL installed on the host machine.

### Installation
1. Clone this repository to your local server.
2. Run the installer script:
   ```bash
   sudo ./install.sh
   ```
   *This script creates a special `lighthouse` user, sets up folders in `/etc/lighthouse`, creates your Certificate Authority, configures Nginx, and starts the Lighthouse service.*

3. Open your web browser and go to:
   `https://lighthouse.internal`

### Installing Runtimes (Go, Java, Node.js)
If your apps need Java, Go, or Node.js to build and run, you can install them using our interactive installer helper:
```bash
sudo lighthouse-install-bins
```
This opens a nice terminal dashboard where you can check/uncheck runtimes and install them automatically.

---

## 3. What is Missing (Future Features list)

Lighthouse is a simple tool, so it does not have everything yet. Here is a list of features we hope to add:

- **Environment Variable Editor:** You currently have to SSH into the server and edit the `.env` files in `/etc/lighthouse/envs/` manually. We need a way to edit them from the web dashboard.
- **Build Log Streaming:** When an app is building, you just see a spinner. If the build fails, you only see the error afterwards. We want to see the terminal output of `go build` or `npm install` live in the browser.
- **CPU and RAM Monitoring:** We want charts on the dashboard showing how much memory and CPU each of your apps is using.
- **Support for more languages:** Right now, we only support **Go**, **Spring Boot (Java)**, and **Next.js**. We want to add support for Python, PHP, Ruby, and static HTML sites soon.
