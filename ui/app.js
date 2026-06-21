function app() {
  return {
    apps: [],
    loading: true,
    alert: { msg: '', type: '' },
    showCreate: false,
    creating: false,
    selectedApp: null,
    tab: 'logs',
    logs: '',
    auditLogs: [],
    nginxHistory: [],
    form: { name: '', type: '', domain: '', port: '', git_url: '', app_dir: '', entry_point: '' },

    async init() {
      await this.loadApps()
    },

    async loadApps() {
      this.loading = true
      try {
        const r = await fetch('/api/apps')
        this.apps = await r.json() || []
      } catch (e) { this.showAlert('Failed to load apps', 'err') }
      this.loading = false
    },

    async openCreate() {
      this.form = { name: '', type: '', domain: '', port: '', git_url: '', app_dir: '', entry_point: '' }
      this.showCreate = true
      try {
        const r = await fetch('/api/apps/next-port')
        const data = await r.json()
        if (r.ok && data.port) {
          this.form.port = data.port
        }
      } catch (e) { console.error("failed to get next port", e) }
    },

    async createApp() {
      if (!this.form.name || !this.form.type || !this.form.domain || !this.form.git_url) {
        this.showAlert('Name, type, domain and git URL are required', 'err'); return
      }
      this.creating = true
      try {
        const r = await fetch('/api/apps', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.form)
        })
        const data = await r.json()
        if (!r.ok) { this.showAlert(data.error || 'Failed to create', 'err'); return }
        this.showCreate = false
        this.showAlert(`App "${this.form.name}" registered!`, 'ok')
        await this.loadApps()
      } catch (e) { this.showAlert('Network error', 'err') }
      this.creating = false
    },

    async editApp(name, currentEntryPoint) {
      const newEntryPoint = prompt("Enter relative Go Entry Point (default: main.go):", currentEntryPoint || 'main.go')
      if (newEntryPoint === null) return
      try {
        const r = await fetch(`/api/apps/${name}/entry-point`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ entry_point: newEntryPoint })
        })
        const data = await r.json()
        if (!r.ok) { this.showAlert(data.error || 'Failed to update entry point', 'err'); return }
        this.showAlert(`Go entry point updated to "${newEntryPoint}"!`, 'ok')
        await this.loadApps()
      } catch (e) { this.showAlert('Network error', 'err') }
    },

    async deployApp(name) {
      this.showAlert(`Deploying ${name}...`, 'ok')
      try {
        const r = await fetch(`/api/apps/${name}/deploy`, { method: 'POST' })
        const data = await r.json()
        if (!r.ok) { this.showAlert(data.error, 'err'); return }
        this.showAlert(`✓ ${name} deployed at https://${this.apps.find(a => a.name === name)?.domain}`, 'ok')
        await this.loadApps()
      } catch (e) { this.showAlert('Deploy failed', 'err') }
    },

    async controlApp(name, action) {
      try {
        const r = await fetch(`/api/apps/${name}/${action}`, { method: 'POST' })
        const data = await r.json()
        if (!r.ok) { this.showAlert(data.error, 'err'); return }
        this.showAlert(`${name} ${action}ed`, 'ok')
        await this.loadApps()
      } catch (e) { this.showAlert('Action failed', 'err') }
    },

    async openDetail(app) {
      this.selectedApp = app
      this.tab = 'logs'
      await this.loadLogs()
    },

    async loadLogs() {
      try {
        const r = await fetch(`/api/apps/${this.selectedApp.name}/logs?lines=150`)
        const data = await r.json()
        this.logs = data.logs || 'No logs.'
      } catch (e) { this.logs = 'Failed to load logs.' }
    },

    async loadAudit() {
      try {
        const r = await fetch(`/api/apps/${this.selectedApp.name}/audit?limit=50`)
        this.auditLogs = await r.json() || []
      } catch (e) { this.auditLogs = [] }
    },

    async loadNginxHistory() {
      try {
        const r = await fetch(`/api/apps/${this.selectedApp.name}/nginx/history`)
        this.nginxHistory = await r.json() || []
      } catch (e) { this.nginxHistory = [] }
    },

    async rollback(hash) {
      if (!confirm(`Restore nginx config to ${hash}?`)) return
      const r = await fetch(`/api/apps/${this.selectedApp.name}/nginx/rollback`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ hash })
      })
      const data = await r.json()
      if (!r.ok) { this.showAlert(data.error, 'err'); return }
      this.showAlert('Nginx config rolled back', 'ok')
      await this.loadNginxHistory()
    },

    statusClass(s) {
      return { 'badge-running': s === 'running', 'badge-stopped': s === 'stopped', 'badge-failed': s === 'failed', 'badge-pending': !s || s === 'pending' }
    },

    formatDate(d) {
      if (!d) return ''
      return new Date(d).toLocaleString('en-IN', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
    },

    showAlert(msg, type) {
      this.alert = { msg, type }
      setTimeout(() => { this.alert = { msg: '', type: '' } }, 4000)
    }
  }
}
