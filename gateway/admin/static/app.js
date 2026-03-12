// GoClaw Admin - Alpine.js Application

function app() {
    return {
        currentPage: 'dashboard',
        navItems: [
            { id: 'dashboard', label: 'Dashboard', icon: '#' },
            { id: 'config', label: 'Config', icon: '*' },
            { id: 'agents', label: 'Agents', icon: '@' },
            { id: 'sessions', label: 'Sessions', icon: '>' },

            { id: 'swarm', label: 'Swarm', icon: '&' },
            { id: 'logs', label: 'Logs', icon: '$' },
            { id: 'cron', label: 'Cron', icon: '%' },
        ],
    };
}

// --- Helper functions ---

function formatBytes(bytes) {
    if (!bytes) return '-';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let val = bytes;
    while (val >= 1024 && i < units.length - 1) {
        val /= 1024;
        i++;
    }
    return val.toFixed(1) + ' ' + units[i];
}

function formatTime(t) {
    if (!t) return '-';
    try {
        return new Date(t).toLocaleString('zh-CN');
    } catch {
        return t;
    }
}

function formatLogTime(t) {
    if (!t) return '';
    try {
        const d = new Date(t);
        return d.toLocaleTimeString('zh-CN', { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0');
    } catch {
        return '';
    }
}

async function apiFetch(path, options = {}) {
    const resp = await fetch('/admin/api/' + path, {
        headers: { 'Content-Type': 'application/json', ...options.headers },
        ...options,
    });
    return resp.json();
}

// --- Page Components ---

function dashboard() {
    return {
        data: {},
        timer: null,
        async init() {
            await this.load();
            this.timer = setInterval(() => this.load(), 10000);
        },
        destroy() {
            if (this.timer) clearInterval(this.timer);
        },
        async load() {
            this.data = await apiFetch('dashboard');
        },
    };
}

function configEditor() {
    return {
        tab: 'llm',
        configText: '',
        configPath: '',
        configMap: {},
        dirty: false,
        error: '',
        success: '',

        // LLM Tab 表单字段
        agentDefaults: { model: '', temperature: 0, max_tokens: 0, max_iterations: 0, max_history_messages: 0 },
        decisionAgent: { enabled: false, provider: '', model: '', api_key: '', base_url: '', timeout_ms: 0, max_tokens: 0, temperature: 0 },
        providers: {
            openrouter: { api_key: '', base_url: '', timeout: 0 },
            openai: { api_key: '', base_url: '', timeout: 0 },
            anthropic: { api_key: '', base_url: '', timeout: 0 },
        },
        failover: { enabled: false, strategy: '' },

        // Channels Tab 表单字段
        channels: {
            telegram:  { enabled: false, token: '', allowed_ids: '' },
            whatsapp:  { enabled: false, bridge_url: '', allowed_ids: '' },
            feishu:    { enabled: false, app_id: '', app_secret: '', encrypt_key: '', verification_token: '', webhook_port: 0, domain: '', group_policy: '', dm_policy: '', allowed_ids: '' },
            qq:        { enabled: false, app_id: '', app_secret: '', allowed_ids: '' },
            wework:    { enabled: false, corp_id: '', agent_id: '', secret: '', token: '', encoding_aes_key: '', webhook_port: 0, allowed_ids: '' },
            dingtalk:  { enabled: false, client_id: '', secret: '', allowed_ids: '' },
            infoflow:  { enabled: false, webhook_url: '', token: '', aes_key: '', webhook_port: 0, allowed_ids: '' },
            gotify:    { enabled: false, server_url: '', app_token: '', priority: 0, allowed_ids: '' },
            weibo:     { enabled: false, app_id: '', app_secret: '', ws_endpoint: '', token_endpoint: '', dm_policy: '', allow_from: '', text_chunk_limit: 0, chunk_mode: '' },
        },
        channelOpen: {},

        // Provider 折叠
        providerOpen: {},

        async init() {
            await this.loadConfig();
        },

        switchTab(t) {
            this.tab = t;
            if (t === 'json') {
                this.configText = JSON.stringify(this.configMap, null, 2);
            }
        },

        async loadConfig() {
            this.error = '';
            this.success = '';
            const resp = await apiFetch('config');
            if (resp.error) {
                this.error = resp.error;
                return;
            }
            this.configPath = resp.config_path || '';
            this.configMap = resp.config || {};
            this.configText = JSON.stringify(this.configMap, null, 2);
            this._destructure();
            this.dirty = false;
        },

        _destructure() {
            const c = this.configMap;
            // Agent Defaults
            const ad = c.agents?.defaults || {};
            this.agentDefaults = {
                model: ad.model || '',
                temperature: ad.temperature || 0,
                max_tokens: ad.max_tokens || 0,
                max_iterations: ad.max_iterations || 0,
                max_history_messages: ad.max_history_messages || 0,
            };
            // Decision Agent
            const da = c.agents?.decision_agent || {};
            this.decisionAgent = {
                enabled: da.enabled || false,
                provider: da.provider || '',
                model: da.model || '',
                api_key: da.api_key || '',
                base_url: da.base_url || '',
                timeout_ms: da.timeout_ms || 0,
                max_tokens: da.max_tokens || 0,
                temperature: da.temperature || 0,
            };
            // Providers
            for (const name of ['openrouter', 'openai', 'anthropic']) {
                const p = c.providers?.[name] || {};
                this.providers[name] = {
                    api_key: p.api_key || '',
                    base_url: p.base_url || '',
                    timeout: p.timeout || 0,
                };
            }
            // Failover
            const fo = c.providers?.failover || {};
            this.failover = { enabled: fo.enabled || false, strategy: fo.strategy || '' };

            // Channels
            const channelDefs = {
                telegram:  ['token', 'allowed_ids'],
                whatsapp:  ['bridge_url', 'allowed_ids'],
                feishu:    ['app_id', 'app_secret', 'encrypt_key', 'verification_token', 'webhook_port', 'domain', 'group_policy', 'dm_policy', 'allowed_ids'],
                qq:        ['app_id', 'app_secret', 'allowed_ids'],
                wework:    ['corp_id', 'agent_id', 'secret', 'token', 'encoding_aes_key', 'webhook_port', 'allowed_ids'],
                dingtalk:  ['client_id', 'secret', 'allowed_ids'],
                infoflow:  ['webhook_url', 'token', 'aes_key', 'webhook_port', 'allowed_ids'],
                gotify:    ['server_url', 'app_token', 'priority', 'allowed_ids'],
                weibo:     ['app_id', 'app_secret', 'ws_endpoint', 'token_endpoint', 'dm_policy', 'allow_from', 'text_chunk_limit', 'chunk_mode'],
            };
            for (const [ch, fields] of Object.entries(channelDefs)) {
                const src = c.channels?.[ch] || {};
                const obj = { enabled: src.enabled || false };
                for (const f of fields) {
                    const val = src[f];
                    if (Array.isArray(val)) {
                        obj[f] = val.join(', ');
                    } else {
                        obj[f] = val ?? this.channels[ch]?.[f] ?? '';
                    }
                }
                this.channels[ch] = obj;
            }
        },

        _assemble() {
            const c = JSON.parse(JSON.stringify(this.configMap));
            // Agent Defaults
            if (!c.agents) c.agents = {};
            if (!c.agents.defaults) c.agents.defaults = {};
            Object.assign(c.agents.defaults, {
                model: this.agentDefaults.model,
                temperature: parseFloat(this.agentDefaults.temperature) || 0,
                max_tokens: parseInt(this.agentDefaults.max_tokens) || 0,
                max_iterations: parseInt(this.agentDefaults.max_iterations) || 0,
                max_history_messages: parseInt(this.agentDefaults.max_history_messages) || 0,
            });
            // Decision Agent
            if (!c.agents.decision_agent) c.agents.decision_agent = {};
            Object.assign(c.agents.decision_agent, {
                enabled: this.decisionAgent.enabled,
                provider: this.decisionAgent.provider,
                model: this.decisionAgent.model,
                api_key: this.decisionAgent.api_key,
                base_url: this.decisionAgent.base_url,
                timeout_ms: parseInt(this.decisionAgent.timeout_ms) || 0,
                max_tokens: parseInt(this.decisionAgent.max_tokens) || 0,
                temperature: parseFloat(this.decisionAgent.temperature) || 0,
            });
            // Providers
            if (!c.providers) c.providers = {};
            for (const name of ['openrouter', 'openai', 'anthropic']) {
                if (!c.providers[name]) c.providers[name] = {};
                Object.assign(c.providers[name], {
                    api_key: this.providers[name].api_key,
                    base_url: this.providers[name].base_url,
                    timeout: parseInt(this.providers[name].timeout) || 0,
                });
            }
            // Failover
            if (!c.providers.failover) c.providers.failover = {};
            Object.assign(c.providers.failover, {
                enabled: this.failover.enabled,
                strategy: this.failover.strategy,
            });
            // Channels
            if (!c.channels) c.channels = {};
            const arrayFields = ['allowed_ids', 'allow_from'];
            const intFields = ['webhook_port', 'priority', 'text_chunk_limit'];
            for (const [ch, form] of Object.entries(this.channels)) {
                if (!c.channels[ch]) c.channels[ch] = {};
                for (const [k, v] of Object.entries(form)) {
                    if (arrayFields.includes(k)) {
                        c.channels[ch][k] = typeof v === 'string' ? v.split(',').map(s => s.trim()).filter(Boolean) : v;
                    } else if (intFields.includes(k)) {
                        c.channels[ch][k] = parseInt(v) || 0;
                    } else if (k === 'enabled') {
                        c.channels[ch][k] = !!v;
                    } else {
                        c.channels[ch][k] = v;
                    }
                }
            }
            return c;
        },

        async saveConfig() {
            this.error = '';
            this.success = '';
            let body;
            if (this.tab === 'json') {
                try {
                    const parsed = JSON.parse(this.configText);
                    body = JSON.stringify(parsed);
                    this.configMap = parsed;
                    this._destructure();
                } catch (e) {
                    this.error = 'Invalid JSON: ' + e.message;
                    return;
                }
            } else {
                const assembled = this._assemble();
                body = JSON.stringify(assembled);
                this.configMap = assembled;
                this.configText = JSON.stringify(assembled, null, 2);
            }
            const resp = await apiFetch('config', {
                method: 'PUT',
                body: body,
            });
            if (resp.error) {
                this.error = resp.error;
            } else {
                this.success = resp.message || 'Config saved';
                this.dirty = false;
            }
        },

        markDirty() {
            this.dirty = true;
        },

        // Channel 表单字段定义
        channelFields(name) {
            const defs = {
                telegram:  [{ k: 'token', l: 'Token', t: 'password' }, { k: 'allowed_ids', l: 'Allowed IDs', t: 'text' }],
                whatsapp:  [{ k: 'bridge_url', l: 'Bridge URL', t: 'text' }, { k: 'allowed_ids', l: 'Allowed IDs', t: 'text' }],
                feishu:    [
                    { k: 'app_id', l: 'App ID', t: 'text' }, { k: 'app_secret', l: 'App Secret', t: 'password' },
                    { k: 'encrypt_key', l: 'Encrypt Key', t: 'password' }, { k: 'verification_token', l: 'Verification Token', t: 'password' },
                    { k: 'webhook_port', l: 'Webhook Port', t: 'number' }, { k: 'domain', l: 'Domain', t: 'text' },
                    { k: 'group_policy', l: 'Group Policy', t: 'text' }, { k: 'dm_policy', l: 'DM Policy', t: 'text' },
                    { k: 'allowed_ids', l: 'Allowed IDs', t: 'text' },
                ],
                qq:        [{ k: 'app_id', l: 'App ID', t: 'text' }, { k: 'app_secret', l: 'App Secret', t: 'password' }, { k: 'allowed_ids', l: 'Allowed IDs', t: 'text' }],
                wework:    [
                    { k: 'corp_id', l: 'Corp ID', t: 'text' }, { k: 'agent_id', l: 'Agent ID', t: 'text' },
                    { k: 'secret', l: 'Secret', t: 'password' }, { k: 'token', l: 'Token', t: 'password' },
                    { k: 'encoding_aes_key', l: 'Encoding AES Key', t: 'password' }, { k: 'webhook_port', l: 'Webhook Port', t: 'number' },
                    { k: 'allowed_ids', l: 'Allowed IDs', t: 'text' },
                ],
                dingtalk:  [{ k: 'client_id', l: 'Client ID', t: 'text' }, { k: 'secret', l: 'Secret', t: 'password' }, { k: 'allowed_ids', l: 'Allowed IDs', t: 'text' }],
                infoflow:  [
                    { k: 'webhook_url', l: 'Webhook URL', t: 'text' }, { k: 'token', l: 'Token', t: 'password' },
                    { k: 'aes_key', l: 'AES Key', t: 'password' }, { k: 'webhook_port', l: 'Webhook Port', t: 'number' },
                    { k: 'allowed_ids', l: 'Allowed IDs', t: 'text' },
                ],
                gotify:    [
                    { k: 'server_url', l: 'Server URL', t: 'text' }, { k: 'app_token', l: 'App Token', t: 'password' },
                    { k: 'priority', l: 'Priority', t: 'number' }, { k: 'allowed_ids', l: 'Allowed IDs', t: 'text' },
                ],
                weibo:     [
                    { k: 'app_id', l: 'App ID', t: 'text' }, { k: 'app_secret', l: 'App Secret', t: 'password' },
                    { k: 'ws_endpoint', l: 'WS Endpoint', t: 'text' }, { k: 'token_endpoint', l: 'Token Endpoint', t: 'text' },
                    { k: 'dm_policy', l: 'DM Policy', t: 'text' }, { k: 'allow_from', l: 'Allow From', t: 'text' },
                    { k: 'text_chunk_limit', l: 'Text Chunk Limit', t: 'number' }, { k: 'chunk_mode', l: 'Chunk Mode', t: 'text' },
                ],
            };
            return defs[name] || [];
        },
    };
}

function agentList() {
    return {
        agents: [],
        selected: null,
        tab: 'identity',
        files: {},
        fileContent: '',
        dirty: false,
        saving: false,
        error: '',
        success: '',

        async init() {
            await this.loadAgents();
        },

        async loadAgents() {
            const resp = await apiFetch('agents');
            this.agents = resp.agents || [];
        },

        async selectAgent(agent) {
            this.selected = agent;
            this.tab = 'identity';
            this.dirty = false;
            this.error = '';
            this.success = '';
            await this.loadFiles();
            await this.loadFile('IDENTITY.md');
        },

        back() {
            if (this.dirty && !confirm('有未保存的修改，确定返回？')) return;
            this.selected = null;
            this.fileContent = '';
            this.dirty = false;
            this.error = '';
            this.success = '';
        },

        async loadFiles() {
            const resp = await apiFetch('agents/' + encodeURIComponent(this.selected.id) + '/files');
            const map = {};
            for (const f of (resp.files || [])) {
                map[f.name] = f;
            }
            this.files = map;
        },

        fileForTab(t) {
            const map = { identity: 'IDENTITY.md', memory: 'MEMORY.md', soul: 'SOUL.md', user: 'USER.md', agents_md: 'AGENTS.md' };
            return map[t] || '';
        },

        async switchTab(t) {
            if (this.dirty && !confirm('有未保存的修改，切换将丢失。继续？')) return;
            this.tab = t;
            this.dirty = false;
            this.error = '';
            this.success = '';
            const file = this.fileForTab(t);
            if (file) {
                await this.loadFile(file);
            }
        },

        async loadFile(name) {
            const resp = await apiFetch('agents/' + encodeURIComponent(this.selected.id) + '/files/' + encodeURIComponent(name));
            this.fileContent = resp.content || '';
            this.dirty = false;
        },

        markDirty() {
            this.dirty = true;
            this.success = '';
        },

        async saveFile() {
            this.saving = true;
            this.error = '';
            this.success = '';
            const file = this.fileForTab(this.tab);
            try {
                const resp = await apiFetch('agents/' + encodeURIComponent(this.selected.id) + '/files/' + encodeURIComponent(file), {
                    method: 'PUT',
                    body: JSON.stringify({ content: this.fileContent }),
                });
                if (resp.error) {
                    this.error = resp.error;
                } else {
                    this.success = file + ' saved';
                    this.dirty = false;
                    await this.loadFiles();
                }
            } catch (e) {
                this.error = e.message;
            }
            this.saving = false;
        },

        tabLabel(t) {
            const labels = { identity: 'Profile', memory: 'Memory', soul: 'Soul', user: 'User', agents_md: 'Agents' };
            return labels[t] || t;
        },

        tabDesc(t) {
            const descs = {
                identity: 'IDENTITY.md - Agent 身份与人设',
                memory: 'MEMORY.md - 长期记忆',
                soul: 'SOUL.md - 原则与价值观',
                user: 'USER.md - 用户信息',
                agents_md: 'AGENTS.md - Agent 配置文档',
            };
            return descs[t] || '';
        },

        fileExists(t) {
            const file = this.fileForTab(t);
            return this.files[file]?.exists || false;
        },
    };
}

function sessionList() {
    return {
        sessions: [],
        selectedSession: null,
        async init() {
            await this.loadSessions();
        },
        async loadSessions() {
            const resp = await apiFetch('sessions');
            this.sessions = resp.sessions || [];
        },
        async viewSession(key) {
            const resp = await apiFetch('sessions/' + encodeURIComponent(key));
            if (!resp.error) {
                this.selectedSession = resp;
            }
        },
        async clearSession(key) {
            if (!key) return;
            if (!confirm('Clear session: ' + key + '?')) return;
            await apiFetch('sessions/' + encodeURIComponent(key), { method: 'DELETE' });
            this.selectedSession = null;
            await this.loadSessions();
        },
    };
}

function channelStatus() {
    return {
        channels: [],
        async init() {
            const resp = await apiFetch('channels');
            this.channels = resp.channels || [];
        },
    };
}

function logViewer() {
    return {
        logs: [],
        connected: false,
        levelFilter: '',
        eventSource: null,
        maxLogs: 2000,
        init() {
            this.connect();
        },
        destroy() {
            if (this.eventSource) {
                this.eventSource.close();
            }
        },
        connect() {
            if (this.eventSource) {
                this.eventSource.close();
            }
            let url = '/admin/api/logs/stream';
            if (this.levelFilter) {
                url += '?level=' + this.levelFilter;
            }
            this.eventSource = new EventSource(url);
            this.eventSource.onopen = () => {
                this.connected = true;
            };
            this.eventSource.onmessage = (e) => {
                try {
                    const entry = JSON.parse(e.data);
                    this.logs.push(entry);
                    // 限制最大条数
                    if (this.logs.length > this.maxLogs) {
                        this.logs = this.logs.slice(-this.maxLogs);
                    }
                    // 自动滚动到底部
                    this.$nextTick(() => {
                        const container = document.getElementById('log-container');
                        if (container) {
                            container.scrollTop = container.scrollHeight;
                        }
                    });
                } catch {}
            };
            this.eventSource.onerror = () => {
                this.connected = false;
                // 5秒后重连
                setTimeout(() => this.connect(), 5000);
            };
        },
        reconnect() {
            this.connect();
        },
    };
}

function cronManager() {
    return {
        jobs: [],
        status: null,
        async init() {
            await this.loadJobs();
        },
        async loadJobs() {
            const resp = await apiFetch('cron');
            this.jobs = resp.jobs || [];
            this.status = resp.status;
        },
        async runJob(id) {
            await apiFetch('cron/' + id + '/run', { method: 'POST' });
            // 短暂延迟后刷新
            setTimeout(() => this.loadJobs(), 1000);
        },
        async deleteJob(id) {
            if (!confirm('Delete job: ' + id + '?')) return;
            await apiFetch('cron/' + id, { method: 'DELETE' });
            await this.loadJobs();
        },
        formatSchedule(schedule) {
            if (!schedule) return '-';
            if (schedule.type === 'cron' && schedule.cron_expression) return schedule.cron_expression;
            if (schedule.type === 'interval' && schedule.every_duration) return 'every ' + (schedule.every_duration / 1e9) + 's';
            if (schedule.type === 'once' && schedule.at) return 'once at ' + formatTime(schedule.at);
            return schedule.type || '-';
        },
    };
}

function swarmManager() {
    return {
        tab: 'configs',
        configs: [],
        active: null,
        tasks: [],
        messages: [],
        approvals: { pending: [], resolved: [] },
        timer: null,
        async init() {
            await this.loadConfigs();
            await this.loadActive();
            this.timer = setInterval(() => {
                if (this.tab === 'active') this.loadActive();
                else if (this.tab === 'messages') this.loadMessages();
                else if (this.tab === 'tasks') this.loadTasks();
                else if (this.tab === 'approvals') this.loadApprovals();
            }, 5000);
        },
        destroy() {
            if (this.timer) clearInterval(this.timer);
        },
        async loadConfigs() {
            const resp = await apiFetch('swarms');
            this.configs = resp.swarms || [];
        },
        async loadActive() {
            const resp = await apiFetch('swarms/active');
            this.active = resp;
        },
        async loadMessages() {
            const resp = await apiFetch('swarms/messages?limit=200');
            this.messages = resp.messages || [];
        },
        async loadTasks() {
            const resp = await apiFetch('swarms/tasks');
            this.tasks = resp.tasks || [];
        },
        async loadApprovals() {
            const resp = await apiFetch('swarms/approvals');
            this.approvals = {
                pending: resp.pending || [],
                resolved: resp.resolved || [],
            };
        },
        async approve(id) {
            await apiFetch('swarms/approvals/' + id + '/approve', { method: 'POST' });
            await this.loadApprovals();
        },
        async reject(id) {
            const reason = prompt('Rejection reason (optional):');
            await apiFetch('swarms/approvals/' + id + '/reject', {
                method: 'POST',
                body: JSON.stringify({ reason: reason || '' }),
            });
            await this.loadApprovals();
        },
        switchTab(t) {
            this.tab = t;
            if (t === 'configs') this.loadConfigs();
            else if (t === 'active') this.loadActive();
            else if (t === 'messages') this.loadMessages();
            else if (t === 'tasks') this.loadTasks();
            else if (t === 'approvals') this.loadApprovals();
        },
        truncateContent(text, maxLen) {
            if (!text) return '';
            if (text.length <= maxLen) return text;
            return text.substring(0, maxLen) + '...';
        },
        taskStatusColor(status) {
            const colors = {
                pending: 'bg-gray-600 text-gray-300',
                approval: 'bg-yellow-900/50 text-yellow-400',
                approved: 'bg-emerald-900/50 text-emerald-400',
                rejected: 'bg-red-900/50 text-red-400',
                assigned: 'bg-blue-900/50 text-blue-400',
                running: 'bg-blue-700/50 text-blue-300',
                review: 'bg-purple-900/50 text-purple-400',
                completed: 'bg-emerald-700/50 text-emerald-300',
                failed: 'bg-red-700/50 text-red-300',
                cancelled: 'bg-gray-700 text-gray-500',
            };
            return colors[status] || 'bg-gray-700 text-gray-400';
        },
    };
}
