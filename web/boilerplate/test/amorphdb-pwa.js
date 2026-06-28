/**
 * AmorphDB PWA Boilerplate
 * Step 1: Section Parser
 * Step 2: Data Tree Generator
 * Step 3: Proxy Wrapper
 * Step 4: Initial DOM Renderer
 * Step 5: Surgical DOM Updates
 * Step 6: List Rendering
 * Step 7: Batch Sender
 * Step 8: SSE Receiver
 * Step 9: Local Watchers
 * Step 10: Deep Link Mapper
 * Step 11: List Windowing
 * Step 12: Shown/Hidden and Focused Updates
 * Step 13: Local Development Mode
 * PWA Auth Step B1: Device ID generation, persistence, and routing
 * PWA Auth Step B2: Token storage, header presentation, SSE URL embedding,
 *                   and 401-driven token clearing
 * PWA Auth Step B3: Login/signup section visibility, login form submission
 *                   to devices.<deviceId>.login, and login error display
 * PWA Auth Step B4: Logout flow — disconnect SSE when the token is cleared
 *                   so a stale connection doesn't linger after the per-user
 *                   logout watcher removes the token server-side. The
 *                   logout intent itself (e.g., `data.intent.logout = true`)
 *                   travels through the existing data-tree write path
 *                   (proxy → flushChanges → POST). The next POST after the
 *                   watcher purges the token returns 401, at which point
 *                   the existing B2 handling clears the local token and
 *                   B3's applyAuthVisibility flips back to the login
 *                   screen. The device ID is preserved so the same browser
 *                   can re-authenticate as a different (or the same) user.
 * PWA Auth Step B5: Graceful 401 / token-expiry handling. Three additions
 *                   on top of B2's MCP POST 401 path: (1) SSE failures are
 *                   probed against /mcp once per disconnect — if the probe
 *                   returns 401, the token is cleared and the login screen
 *                   shown; otherwise normal exponential backoff continues.
 *                   (2) Changes that were queued in flushChanges when the
 *                   401 arrived are snapshotted to `pendingChanges` so the
 *                   pre-auth POSTs that follow (login/signup) don't carry
 *                   stale post-auth writes; on a successful re-login,
 *                   setToken merges them back and re-flushes. (3) Auth
 *                   transitions log a single user-friendly line ("Session
 *                   ended — please log in again") rather than dumping a
 *                   stack trace, since 401 is an expected outcome of
 *                   normal token expiry, not a bug.
 */

class AmorphPWA {
    constructor() {
        this.sectionTree = null;
        this.dataTree = null;
        this.rawData = null;  // Store raw data for direct server updates
        this.proxiedData = null;  // The proxy-wrapped data tree
        this.changes = new Map();  // Track changes for server sync
        this.sendScheduled = false;  // Flag for microtask scheduling
        this.proxyCache = new WeakMap();  // Cache proxies to avoid creating duplicates
        this.listTemplates = new Map();  // Store list item templates for cloning
        this.listContainers = new Map();  // Store references to list container DOM elements
        this.appContainer = null;  // Store reference to this PWA's main app container element

        // SSE connection properties
        this.eventSource = null;  // Current EventSource connection
        this.reconnectAttempts = 0;  // Track reconnection attempts for backoff
        this.maxReconnectAttempts = 10;  // Max reconnection attempts before giving up
        this.maxReconnectDelay = 30000;  // Max 30 seconds between reconnect attempts
        this.lastUpdateTime = localStorage.getItem('amorphdb_last_update') || null;  // Last update timestamp

        // Local watcher system
        this.watchers = new Map();  // Map of paths to arrays of callbacks

        // Step 12: Focused updates - keepLive path registration
        this.keepLivePaths = new Set();  // Set of paths that should always receive SSE updates

        // Step 13: Local Development Mode
        this.localDevMode = false;  // Flag indicating whether we're running in local development mode

        // PWA Auth Step B1: Device ID — generated on first load, persisted in
        // localStorage as `amorphdb_device_id`. Sent as the X-AmorphDB-Device
        // header on every MCP POST and embedded in the SSE connection URL so
        // the Go bridge can route pre-auth writes to
        // world.apps.<app>.devices.<deviceId>.* and SSE events back to the
        // originating browser. The device ID identifies the browser, not the
        // person, and survives page refreshes and browser restarts.
        this.deviceId = this.getOrCreateDeviceId();

        // PWA Auth Step B2: Auth token — stored in localStorage as
        // `amorphdb_token` after a successful login response and presented as
        // the X-AmorphDB-Token header on subsequent MCP POSTs and as
        // `?token=<token>` in the SSE URL. The Go bridge resolves the token
        // to an identity (Step A2) and routes writes to
        // world.apps.<app>.users.<identity>.*. A 401 response from any POST
        // causes the token to be cleared. Pre-auth (no token) is the initial
        // state; login (Step B3) populates it via setToken().
        this.token = this.getStoredToken();

        // PWA Auth Step B3: Track which auth form (`login` or `signup`) was
        // most recently submitted so handleLoginResponse can write any error
        // message back to that form's `.error` field. Also a flag indicating
        // whether the section tree contains login/signup sections, set by
        // applyAuthVisibility on first call. Apps with no auth sections
        // (guest mode) leave hasAuthSections false and skip the visibility
        // toggling entirely.
        this.pendingAuthForm = null;
        this.hasAuthSections = false;

        // PWA Auth Step B5: Changes that were queued when a 401 arrived.
        // flushChanges snapshots `this.changes` into this Map before
        // clearing it (so a follow-up pre-auth POST doesn't mix stale
        // post-auth writes with fresh login/signup credentials), and
        // setToken merges them back on a successful re-login so the
        // user doesn't lose work to a token expiry mid-edit. Empty
        // until the first 401.
        this.pendingChanges = new Map();

        // PWA Auth Step B5: Tracks whether the SSE auth probe has run
        // since the last successful EventSource onopen. handleSSEError
        // probes /mcp at most once per disconnect so a 401-vs-network
        // ambiguity gets resolved without spamming the bridge on every
        // backoff attempt. Reset to false in connectSSE's onopen.
        this.sseProbedSinceLastOpen = false;

        console.log('AmorphDB PWA Boilerplate initialized');
    }

    /**
     * Read the auth token from localStorage. Returns null if absent or if
     * localStorage is unavailable (e.g., sandboxed test contexts).
     * @returns {string|null} The stored token, or null if none
     */
    getStoredToken() {
        try {
            return localStorage.getItem('amorphdb_token') || null;
        } catch (e) {
            return null;
        }
    }

    /**
     * Persist a freshly issued auth token. Called after a successful login
     * response surfaces `result.token`. Subsequent MCP POSTs and SSE
     * connections will present the token until it expires or is cleared.
     *
     * PWA Auth Step B3: After persisting the token, flip section visibility
     * so login/signup sections hide and the rest of the app shows.
     *
     * PWA Auth Step B5: After a successful (re-)login, retry any changes
     * that were preserved by flushChanges when the token expired. The
     * user's in-flight edit was held in pendingChanges precisely so it
     * could resume here.
     * @param {string} token - The auth token from the login watcher
     */
    setToken(token) {
        this.token = token;
        try {
            localStorage.setItem('amorphdb_token', token);
        } catch (e) {
            // localStorage unavailable — token still works in-memory for the
            // life of the page; reload will revert to pre-auth state.
        }
        this.applyAuthVisibility();
        this.retryPendingChanges();
    }

    /**
     * Merge any preserved-through-401 changes back into the live changes
     * Map and schedule a flush. Called from setToken after a successful
     * re-login. Newer entries in `this.changes` (writes the user made on
     * the login screen pre-auth, if any leaked through) take precedence
     * over the preserved ones; the snapshot only fills in paths that
     * aren't already pending.
     *
     * Fires `amorphPWAPendingChangesRetried` with the count so apps that
     * want to display "your unsaved order has been restored" toasts can.
     *
     * No-op if pendingChanges is empty.
     */
    retryPendingChanges() {
        if (!this.pendingChanges || this.pendingChanges.size === 0) {
            return;
        }

        const retried = this.pendingChanges.size;
        for (const [path, value] of this.pendingChanges) {
            if (!this.changes.has(path)) {
                this.changes.set(path, value);
            }
        }
        this.pendingChanges.clear();

        console.log(`🔁 Retrying ${retried} change(s) preserved through re-login`);
        this.scheduleSend();

        const event = new CustomEvent('amorphPWAPendingChangesRetried', {
            detail: { count: retried }
        });
        document.dispatchEvent(event);
    }

    /**
     * Discard the stored token. Called on 401 (token expired or unknown) and
     * by logout (Step B4). The device ID is preserved so the user can
     * re-authenticate without losing browser identity.
     *
     * PWA Auth Step B3: After clearing the token, flip section visibility
     * so login/signup sections show and the rest of the app hides.
     *
     * PWA Auth Step B4: Tear down the SSE connection so a stale stream
     * keyed by the now-defunct token (Step A3 routing) doesn't linger
     * after logout. The bridge will reject the next reconnect attempt
     * with a 401, but proactively closing client-side avoids that round
     * trip and any briefly-delivered events for the prior identity. A
     * fresh SSE connection opens automatically on the next setToken
     * (handleLoginResponse re-keys to the new identity per Step A3).
     */
    clearToken() {
        this.token = null;
        try {
            localStorage.removeItem('amorphdb_token');
        } catch (e) {
            // ignore — see setToken
        }
        this.disconnectSSE();
        this.applyAuthVisibility();
    }

    /**
     * Close the active SSE EventSource (if any) and reset the reconnect
     * backoff counter. Safe to call when no connection is open.
     *
     * PWA Auth Step B4: Used by clearToken() so a logout / 401 doesn't
     * leave a stale stream open. The next connectSSE() call (e.g., after
     * a successful re-login) starts fresh. handleSSEError's exponential
     * backoff is short-circuited because reconnectAttempts is reset and
     * the closed EventSource won't fire onerror once it's been explicitly
     * closed by the client.
     */
    disconnectSSE() {
        if (this.eventSource) {
            try {
                this.eventSource.close();
            } catch (e) {
                console.error('❌ Error closing SSE connection:', e);
            }
            this.eventSource = null;
        }
        this.reconnectAttempts = 0;
    }

    /**
     * Apply pre-auth/post-auth visibility to top-level sections. Called once
     * after initial DOM render and again whenever the token state changes
     * (setToken/clearToken).
     *
     * Pre-auth (no token): top-level sections named `login` and `signup` are
     * shown; all other top-level sections are hidden.
     * Post-auth (token present): `login` and `signup` are hidden; the rest
     * are shown.
     *
     * If the section tree has no `login` or `signup` sections (guest mode),
     * this is a no-op so the full app stays visible regardless of token
     * state. The visibility transition uses the existing _shown mechanism
     * (Step 12) so updateSectionVisibility handles the actual DOM toggle.
     */
    applyAuthVisibility() {
        if (!this.sectionTree || !this.sectionTree.children) {
            return;
        }

        const childNames = Object.keys(this.sectionTree.children);
        this.hasAuthSections = childNames.some(
            name => name === 'login' || name === 'signup'
        );
        if (!this.hasAuthSections) {
            return;
        }

        const isAuthenticated = this.token !== null && this.token !== undefined;

        childNames.forEach(sectionName => {
            const isAuthSection = (sectionName === 'login' || sectionName === 'signup');
            const shouldShow = isAuthSection ? !isAuthenticated : isAuthenticated;

            if (this.rawData && sectionName in this.rawData) {
                const node = this.rawData[sectionName];
                if (node && typeof node === 'object' && !Array.isArray(node)) {
                    node._shown = shouldShow;
                }
            }
            this.updateField(`${sectionName}._shown`, shouldShow);
        });
    }

    /**
     * Read the device ID from localStorage, or generate and persist a new one
     * on first load. The ID is 32 hex characters of CSPRNG output (16 bytes).
     * @returns {string} The device ID
     */
    getOrCreateDeviceId() {
        let deviceId = null;
        try {
            deviceId = localStorage.getItem('amorphdb_device_id');
        } catch (e) {
            // localStorage may be unavailable in some sandboxed test
            // environments — fall through to generation; the resulting ID
            // won't persist across reloads in that case.
        }

        if (!deviceId || !/^[0-9a-f]{32}$/.test(deviceId)) {
            const bytes = new Uint8Array(16);
            crypto.getRandomValues(bytes);
            deviceId = Array.from(bytes)
                .map(b => b.toString(16).padStart(2, '0'))
                .join('');
            try {
                localStorage.setItem('amorphdb_device_id', deviceId);
            } catch (e) {
                // ignore — see above
            }
        }

        return deviceId;
    }

    /**
     * Parse app.html and build the section tree
     * @param {string} appHtml - The HTML content of app.html
     * @returns {Object} The parsed section tree
     */
    parseApp(appHtml) {
        console.log('Parsing app.html...');

        try {
            // Create a temporary DOM element to parse the HTML
            const parser = new DOMParser();
            const doc = parser.parseFromString(appHtml, 'text/html');

            // Find all top-level sections in the document
            const rootSections = doc.querySelectorAll('section');
            console.log('Found sections:', rootSections.length);

            if (rootSections.length === 0) {
                console.warn('No sections found in app.html');
                return null;
            }

            // Parse the first root section (should be the main app section)
            this.sectionTree = this.parseSection(rootSections[0]);
            console.log('Section tree parsed successfully');
            return this.sectionTree;

        } catch (error) {
            console.error('Error in parseApp():', error);
            throw error;
        }
    }

    /**
     * Parse a single section and its children recursively
     * @param {Element} sectionElement - The section DOM element
     * @returns {Object} The parsed section object
     */
    parseSection(sectionElement) {
        console.log('Parsing section...');

        // Extract section attributes
        const name = sectionElement.getAttribute('name');
        console.log('Section name:', name);

        if (!name) {
            throw new Error('Section missing required "name" attribute');
        }

        const layout = sectionElement.getAttribute('layout') || 'vertical';
        const type = sectionElement.getAttribute('type') || null;
        const showMax = sectionElement.getAttribute('show_max') ?
            parseInt(sectionElement.getAttribute('show_max')) : null;
        const loadMax = sectionElement.getAttribute('load_max') ?
            parseInt(sectionElement.getAttribute('load_max')) : 200;

        console.log('Section attributes:', { name, layout, type, showMax, loadMax });

        // Clone the section to extract template content without modifying original
        const sectionClone = sectionElement.cloneNode(true);

        // Find child sections
        const childSections = Array.from(sectionClone.querySelectorAll(':scope > section'));
        console.log('Found child sections:', childSections.length);

        // Remove child sections from clone to get just template content
        childSections.forEach(child => {
            if (sectionClone.contains(child)) {
                sectionClone.removeChild(child);
            }
        });

        // Extract template content (everything except child sections)
        const templateContent = sectionClone.innerHTML.trim();
        console.log('Template content for', name, ':', templateContent.substring(0, 100));

        // Extract field names from {fieldname} placeholders
        const fieldNames = this.extractFieldNames(templateContent);
        console.log('Extracted fields for', name, ':', fieldNames);

        // Debug banner section specifically
        if (name === 'banner') {
            console.log('BANNER SECTION DEBUG:');
            console.log('Template content:', templateContent);
            console.log('Field names extracted:', fieldNames);
        }

        // Parse child sections recursively
        const children = {};
        childSections.forEach(childElement => {
            console.log('Parsing child section...');
            const childSection = this.parseSection(childElement);
            children[childSection.name] = childSection;
        });

        const result = {
            name: name,
            layout: layout,
            type: type,
            show_max: showMax,
            load_max: loadMax,
            template: templateContent,
            fields: fieldNames,
            children: children
        };

        console.log('Completed parsing section:', name, result);
        return result;
    }

    /**
     * Extract field names from {fieldname} placeholders in template content
     * @param {string} template - The template HTML string
     * @returns {Array} Array of unique field names
     */
    extractFieldNames(template) {
        const fieldRegex = /\{([a-zA-Z_][a-zA-Z0-9_]*)\}/g;
        const fields = new Set();
        let match;

        while ((match = fieldRegex.exec(template)) !== null) {
            fields.add(match[1]);
        }

        return Array.from(fields);
    }

    /**
     * Generate data tree from parsed section tree
     * @param {Object} sectionTree - The parsed section tree
     * @returns {Object} The generated data tree
     */
    generateDataTree(sectionTree = null) {
        console.log('Generating data tree...');

        const tree = sectionTree || this.sectionTree;
        if (!tree) {
            console.warn('No section tree available for data tree generation');
            return null;
        }

        this.dataTree = this.generateDataNode(tree, []);
        console.log('Data tree generated successfully');
        return this.dataTree;
    }

    /**
     * Generate a single data node from a section node recursively
     * @param {Object} section - The section object
     * @param {Array} path - Current path for logging
     * @returns {Object} The generated data node
     */
    generateDataNode(section, path = []) {
        // Null check for section
        if (!section || typeof section !== 'object') {
            console.warn('generateDataNode called with invalid section:', section);
            return {};
        }

        // For list sections, create an empty array with windowing properties
        if (section.type === 'list') {
            const listArray = [];
            // Step 11: List Windowing - add system properties for windowing
            listArray._from = 0;
            listArray._through = 0;
            listArray._total = 0;
            listArray._load_max = section.load_max || 200;
            return listArray;
        }

        const dataNode = {};

        // Add _shown property to every section node (non-list sections only)
        dataNode._shown = true;

        // Add properties for each field in this section's template
        if (section.fields && section.fields.length > 0) {
            section.fields.forEach(fieldName => {
                // Determine default value based on field name patterns
                const defaultValue = this.getDefaultValueForField(fieldName);
                dataNode[fieldName] = defaultValue;
            });
        }

        // Recursively process child sections
        if (section.children && Object.keys(section.children).length > 0) {
            Object.keys(section.children).forEach(childName => {
                const childSection = section.children[childName];
                const childPath = [...path, childName];
                dataNode[childName] = this.generateDataNode(childSection, childPath);
            });
        }

        return dataNode;
    }

    /**
     * Determine default value for a field based on its name
     * @param {string} fieldName - The field name
     * @returns {any} The default value
     */
    getDefaultValueForField(fieldName) {
        // Convert field name to lowercase for pattern matching
        const lowerName = fieldName.toLowerCase();

        // Number fields
        if (lowerName.includes('quantity') || lowerName.includes('amount') ||
            lowerName.includes('price') || lowerName.includes('count') ||
            lowerName.includes('number') || lowerName.includes('max') ||
            lowerName.includes('min')) {
            return 0;
        }

        // Boolean fields
        if (lowerName.includes('submit') || lowerName.includes('active') ||
            lowerName.includes('enabled') || lowerName.includes('visible') ||
            lowerName.includes('checked') || lowerName.includes('selected')) {
            return false;
        }

        // Default to empty string for text fields
        return "";
    }

    /**
     * Create a recursive proxy for change tracking
     * @param {Object} obj - The object to wrap
     * @param {string} path - Current path for tracking
     * @returns {Proxy} The proxied object
     */
    createProxy(obj, path = '') {
        const self = this;

        // Special handling for arrays to intercept list operations
        if (Array.isArray(obj)) {
            return new Proxy(obj, {
                get(target, key) {
                    // Return raw value for system properties starting with _
                    if (key.toString().startsWith('_')) {
                        return target[key];
                    }

                    const value = target[key];

                    // Array methods that modify the array need special handling
                    if (key === 'push') {
                        return function(...items) {
                            const oldLength = target.length;
                            const result = Array.prototype.push.apply(target, items);

                            // Add new items to list DOM
                            for (let i = oldLength; i < target.length; i++) {
                                self.addListItem(path, i, target[i]);
                            }

                            self.changes.set(path, [...target]);
                            self.scheduleSend();
                            return result;
                        };
                    } else if (key === 'pop') {
                        return function() {
                            if (target.length > 0) {
                                const oldLength = target.length;
                                const result = Array.prototype.pop.apply(target);
                                self.removeListItem(path, oldLength - 1);
                                self.changes.set(path, [...target]);
                                self.scheduleSend();
                                return result;
                            }
                        };
                    } else if (key === 'splice') {
                        return function(start, deleteCount, ...items) {
                            const oldLength = target.length;
                            const result = Array.prototype.splice.apply(target, [start, deleteCount, ...items]);

                            // Re-render entire list for splice operations (complex indexing)
                            self.renderListContents(path, target);

                            self.changes.set(path, [...target]);
                            self.scheduleSend();
                            return result;
                        };
                    }

                    // Handle array indices - wrap in proxy for item field updates
                    if (typeof key === 'string' && /^\d+$/.test(key)) {
                        const index = parseInt(key);
                        if (index >= 0 && index < target.length && value !== null && typeof value === 'object') {
                            const itemPath = path ? path + '.' + key : key.toString();

                            // Check cache first
                            if (self.proxyCache.has(value)) {
                                return self.proxyCache.get(value);
                            }

                            const itemProxy = self.createProxy(value, itemPath);
                            self.proxyCache.set(value, itemProxy);
                            return itemProxy;
                        }
                    }

                    return value;
                },

                set(target, key, value) {
                    const fullPath = path ? path + '.' + key : key.toString();

                    // Handle array length changes or direct index assignment
                    if (key === 'length' || (typeof key === 'string' && /^\d+$/.test(key))) {
                        target[key] = value;

                        // If entire array is being replaced, re-render list
                        if (key === 'length' || typeof value === 'object') {
                            self.renderListContents(path, target);
                        }

                        if (!key.toString().startsWith('_')) {
                            self.changes.set(path, [...target]);
                            self.scheduleSend();
                            self.notifyWatchers(path, [...target]);
                        }
                    } else {
                        target[key] = value;

                        if (!key.toString().startsWith('_')) {
                            self.changes.set(fullPath, value);
                            self.scheduleSend();
                            self.updateField(fullPath, value);
                            self.notifyWatchers(fullPath, value);
                        }
                    }

                    return true;
                }
            });
        }

        // Regular object proxy
        return new Proxy(obj, {
            get(target, key) {
                // Return raw value for system properties starting with _
                if (key.toString().startsWith('_')) {
                    return target[key];
                }

                const value = target[key];

                // If value is an object (but not null), wrap it in a proxy too
                if (value !== null && typeof value === 'object') {
                    const newPath = path ? path + '.' + key : key.toString();

                    // Check cache first to avoid creating duplicate proxies
                    if (self.proxyCache.has(value)) {
                        return self.proxyCache.get(value);
                    }

                    const nestedProxy = self.createProxy(value, newPath);
                    self.proxyCache.set(value, nestedProxy);
                    return nestedProxy;
                }

                return value;
            },

            set(target, key, value) {
                const fullPath = path ? path + '.' + key : key.toString();

                // Store the value in the target
                target[key] = value;

                // Track changes for server sync (but not for system properties starting with _)
                if (!key.toString().startsWith('_')) {
                    self.changes.set(fullPath, value);
                    self.scheduleSend();

                    // For array replacement, re-render the list
                    if (Array.isArray(value)) {
                        self.renderListContents(fullPath, value);
                    } else {
                        // Immediately update the DOM (Step 5: Surgical DOM Updates)
                        self.updateField(fullPath, value);
                    }

                    // Notify watchers of the change (Step 9: Local Watchers)
                    self.notifyWatchers(fullPath, value);
                }

                return true;
            }
        });
    }

    /**
     * Schedule sending changes to server using microtask
     */
    scheduleSend() {
        if (!this.sendScheduled) {
            this.sendScheduled = true;
            queueMicrotask(() => {
                this.flushChanges();
            });
        }
    }

    /**
     * Flush accumulated changes to the server
     */
    async flushChanges() {
        // Step 13: In local dev mode, silently skip sending (no error spam in console)
        if (this.localDevMode) {
            this.changes.clear();
            this.sendScheduled = false;
            return;
        }

        // Get changes before clearing
        const changeCount = this.changes.size;
        if (changeCount === 0) {
            this.sendScheduled = false;
            return;
        }

        // Convert changes Map to object for JSON serialization
        const changesObject = {};
        for (const [path, value] of this.changes) {
            changesObject[path] = value;
        }

        // PWA Auth Step B3: If the change set includes a login/signup
        // submission, remember which form was submitted so the response
        // handler can route any error message back to that form's `.error`
        // field. The watcher writes its result to
        // device.<form>.result and the bridge surfaces it as
        // response.result; we then write result.message to data.<form>.error
        // so the section's `{error}` placeholder updates surgically.
        if (changesObject['login.submitted'] === true) {
            this.pendingAuthForm = 'login';
        } else if (changesObject['signup.submitted'] === true) {
            this.pendingAuthForm = 'signup';
        }

        // Build MCP message
        const mcpMessage = {
            type: 'data_update',
            user_id: this.getUserId(),
            changes: changesObject
        };

        console.log(`🚀 Sending batch: ${changeCount} changes to server`, mcpMessage);

        // Notify test interface that a batch is being sent
        this.notifyBatchSent(mcpMessage);

        try {
            // PWA Auth Step B2: Build request headers. Device ID always
            // present (Step B1). Token included only when authenticated;
            // pre-auth login/signup POSTs deliberately omit it.
            const headers = {
                'Content-Type': 'application/json',
                'X-AmorphDB-Device': this.deviceId,
            };
            if (this.token) {
                headers['X-AmorphDB-Token'] = this.token;
            }

            // POST to /mcp endpoint
            const response = await fetch('/mcp', {
                method: 'POST',
                headers: headers,
                body: JSON.stringify(mcpMessage)
            });

            if (response.ok) {
                console.log('✅ Changes sent successfully to server');
                // Clear changes only on successful send
                this.changes.clear();

                // PWA Auth Step B2: Inspect the response for a login result
                // (Step A4 surfaces the watcher's parsed JSON as
                // response.result). On `result.status = "ok"` with a
                // `result.token`, store the token and reconnect SSE so the
                // bridge re-keys the stream from device-only to device+identity
                // (Step A3). Errors and pre-auth POSTs without a result fall
                // through unchanged.
                await this.handleLoginResponse(response);
            } else if (response.status === 401) {
                // PWA Auth Step B2: Token rejected by the bridge (expired,
                // unknown, or device-mismatched per Step A2). Clear the
                // token so the next POST goes pre-auth and notify any
                // listener that the login screen should be shown. The full
                // login-screen routing lands in Step B3; here we only
                // update auth state.
                //
                // PWA Auth Step B5: Snapshot any unsent changes so the
                // user's in-flight edit isn't lost and a follow-up
                // pre-auth POST (login/signup) doesn't carry stale
                // post-auth writes. setToken's retryPendingChanges will
                // re-flush them after a successful re-login.
                this.preservePendingChanges();
                if (this.token) {
                    this.clearToken();
                }
                console.info('🔒 Session ended — please log in again');
                this.notifyAuthRequired();
            } else {
                console.error(`❌ Server responded with error: ${response.status} ${response.statusText}`);
                // Keep changes in map for retry (though we don't implement retry logic here)
            }

        } catch (error) {
            console.error('❌ Network error sending changes to server:', error);
            // Keep changes in map for potential retry
        }

        // Reset the send scheduled flag
        this.sendScheduled = false;
    }

    /**
     * Inspect a successful MCP POST response for a login/signup result and
     * store any returned token. Step A4 of the bridge surfaces the watcher's
     * parsed JSON object as `response.result`. The boilerplate stores the
     * token (`result.token` on `result.status === "ok"`) and reconnects SSE
     * so the stream is re-keyed by identity.
     *
     * The response body is consumed via response.clone().json() so the
     * caller's Response object remains intact for any downstream inspection
     * (e.g., test mocks). JSON parse errors are swallowed — many MCP
     * responses are non-JSON or empty — and treated as "no login result".
     *
     * @param {Response} response - The successful fetch Response
     */
    async handleLoginResponse(response) {
        let body = null;
        try {
            const cloned = typeof response.clone === 'function' ? response.clone() : response;
            body = await cloned.json();
        } catch (e) {
            return;
        }

        if (!body || typeof body !== 'object') {
            return;
        }
        const result = body.result;
        if (!result || typeof result !== 'object') {
            return;
        }

        if (result.status === 'ok' && typeof result.token === 'string' && result.token.length > 0) {
            console.log('🔑 Login successful — storing token and reconnecting SSE');

            // PWA Auth Step B3: Clear any prior error displayed on the
            // login/signup forms so a successful re-login wipes out the
            // earlier error message. Only writes if the field exists in
            // the data tree.
            this.clearAuthFormError('login');
            this.clearAuthFormError('signup');
            this.pendingAuthForm = null;

            this.setToken(result.token);

            const event = new CustomEvent('amorphPWALoginSuccess', {
                detail: { result: result }
            });
            document.dispatchEvent(event);

            try {
                this.connectSSE();
            } catch (e) {
                console.error('❌ Error reconnecting SSE after login:', e);
            }
        } else if (result.status === 'error') {
            console.warn(`🔒 Login error: ${result.message || '(no message)'}`);

            // PWA Auth Step B3: Write the error message to the auth form's
            // `.error` field so any `{error}` placeholder in the login or
            // signup section updates surgically. The form is the one that
            // submitted in this batch (tracked in pendingAuthForm in
            // flushChanges).
            if (this.pendingAuthForm) {
                const message = (typeof result.message === 'string' && result.message.length > 0)
                    ? result.message
                    : 'Login failed';
                this.applyServerUpdate(`${this.pendingAuthForm}.error`, message);
            }
            this.pendingAuthForm = null;

            const event = new CustomEvent('amorphPWALoginError', {
                detail: { result: result }
            });
            document.dispatchEvent(event);
        }
    }

    /**
     * Clear the error message displayed on a given auth form section.
     * Called from handleLoginResponse on a successful login so a prior
     * error doesn't linger after the user re-tries successfully. No-op if
     * the section or field doesn't exist in the data tree.
     * @param {string} formName - Either "login" or "signup"
     */
    clearAuthFormError(formName) {
        if (!this.rawData || typeof this.rawData !== 'object') {
            return;
        }
        const node = this.rawData[formName];
        if (node && typeof node === 'object' && !Array.isArray(node) && 'error' in node) {
            this.applyServerUpdate(`${formName}.error`, '');
        }
    }

    /**
     * Snapshot the live `this.changes` Map into `this.pendingChanges` and
     * empty `this.changes`. Called from the 401 handler in flushChanges
     * (and from probeAuthAfterSSEFailure when the SSE probe confirms the
     * token is invalid) so unsent edits survive a token expiry without
     * being mixed into the pre-auth login submission that follows. Only
     * paths that aren't already pending are saved — if the user has
     * 401'd before and not yet re-logged in, the older snapshot wins.
     *
     * No-op if there are no live changes.
     */
    preservePendingChanges() {
        if (!this.changes || this.changes.size === 0) {
            return;
        }
        for (const [path, value] of this.changes) {
            if (!this.pendingChanges.has(path)) {
                this.pendingChanges.set(path, value);
            }
        }
        this.changes.clear();
    }

    /**
     * Fire a `amorphPWAAuthRequired` event so the login UI (Step B3) can
     * surface the login screen. B2 only signals the need; B3 will register
     * the listener that re-renders the pre-auth sections.
     *
     * PWA Auth Step B5: include the count of preserved changes in the
     * detail so apps can render "your changes are pending — log in to
     * save" without poking at internals.
     */
    notifyAuthRequired() {
        const event = new CustomEvent('amorphPWAAuthRequired', {
            detail: {
                deviceId: this.deviceId,
                pendingChanges: this.pendingChanges ? this.pendingChanges.size : 0
            }
        });
        document.dispatchEvent(event);
    }

    /**
     * Get user ID for MCP messages
     * @returns {string} User ID (placeholder implementation)
     */
    getUserId() {
        // In a real implementation, this would come from authentication
        // For testing, use a fixed ID
        return 'test_user_123';
    }

    /**
     * Notify test interface that a batch is being sent
     * @param {Object} mcpMessage - The MCP message being sent
     */
    notifyBatchSent(mcpMessage) {
        // Fire custom event for test interface
        const event = new CustomEvent('amorphPWABatchSent', {
            detail: {
                message: mcpMessage,
                changeCount: Object.keys(mcpMessage.changes).length,
                timestamp: new Date().toISOString()
            }
        });
        document.dispatchEvent(event);

        // Also update any batch status elements on the page
        const statusElements = document.querySelectorAll('.batch-status');
        statusElements.forEach(element => {
            const timestamp = new Date().toLocaleTimeString();
            element.textContent = `Last batch: ${Object.keys(mcpMessage.changes).length} changes at ${timestamp}`;
        });
    }

    /**
     * Connect to the SSE endpoint for real-time server updates
     * Step 8: SSE Receiver implementation
     *
     * PWA Auth Step B1: SSE path is keyed by device ID. The Go bridge serves
     * the route at `/client-sse/<deviceId>` and routes events back to the
     * originating browser. Pre-auth, this device receives events under
     * `world.apps.<app>.devices.<deviceId>.*`.
     *
     * PWA Auth Step B2: When a token is present, append `?token=<token>` so
     * Step A3 can promote the connection to user-keyed routing
     * (`world.apps.<app>.users.<identity>.*`). EventSource cannot set
     * custom headers, so the token must travel in the URL. Combines with
     * the existing `since=<timestamp>` resync parameter when both apply.
     */
    connectSSE() {
        const baseUrl = `/client-sse/${this.deviceId}`;

        const params = [];
        if (this.token) {
            params.push(`token=${this.token}`);
        }
        if (this.lastUpdateTime) {
            params.push(`since=${this.lastUpdateTime}`);
        }
        const url = params.length > 0 ? `${baseUrl}?${params.join('&')}` : baseUrl;

        console.log(`🔌 Connecting to SSE endpoint: ${url}`);

        try {
            // Close any existing connection
            if (this.eventSource) {
                this.eventSource.close();
            }

            this.eventSource = new EventSource(url);

            this.eventSource.onopen = () => {
                console.log('✅ SSE connection established');
                this.reconnectAttempts = 0;  // Reset backoff counter
                // PWA Auth Step B5: a successful handshake means the
                // token (if any) is valid. Allow the probe to fire
                // again the next time the connection drops.
                this.sseProbedSinceLastOpen = false;
            };

            this.eventSource.onmessage = (event) => {
                console.log('📨 SSE message received:', event.data);

                try {
                    const message = JSON.parse(event.data);

                    // Store timestamp for reconnection
                    if (message.timestamp) {
                        this.lastUpdateTime = message.timestamp;
                        localStorage.setItem('amorphdb_last_update', this.lastUpdateTime);
                    }

                    // Step 11: Check for list page responses
                    if (message.type === 'list_page_response') {
                        this.handleListPageUpdate(message.path, message.items, message.from, message.total);
                        return;
                    }

                    // Apply regular server updates
                    const changes = message.changes || message;  // Support both formats
                    for (const [path, value] of Object.entries(changes)) {
                        this.applyServerUpdate(path, value);
                    }

                } catch (error) {
                    console.error('❌ Error parsing SSE message:', error, event.data);
                }
            };

            this.eventSource.onerror = (error) => {
                console.error('❌ SSE connection error:', error);
                this.handleSSEError();
            };

        } catch (error) {
            console.error('❌ Error creating SSE connection:', error);
            this.handleSSEError();
        }
    }

    /**
     * Apply updates received from the server via SSE
     * @param {string} path - Dot-separated path to update
     * @param {any} value - New value from server
     */
    applyServerUpdate(path, value) {
        console.log(`📥 Server update: ${path} = ${JSON.stringify(value)}`);

        try {
            // Update rawData directly (bypassing proxy to avoid echoing back to server)
            this.setValueAtPath(this.rawData, path, value);

            // Update the DOM to reflect the change
            this.updateField(path, value);

            // Notify any watchers (Step 9: Local Watchers)
            this.notifyWatchers(path, value);

            console.log(`✅ Applied server update for ${path}`);

        } catch (error) {
            console.error(`❌ Error applying server update for ${path}:`, error);
        }
    }

    /**
     * Handle SSE connection errors and implement reconnection with exponential backoff.
     *
     * PWA Auth Step B5: When a token is set and we haven't yet probed
     * since the last successful onopen, send a single auth-check POST
     * to /mcp before scheduling the next reconnect. If the bridge
     * returns 401, the token is invalid (expired, purged by logout,
     * device-mismatched per Step A2) — clear it and surface the login
     * screen instead of looping through 10 backoff attempts that all
     * 401. If the probe returns anything else (200, 5xx, network
     * error), assume the SSE failure was transient and continue with
     * normal exponential backoff.
     *
     * Returns the promise from the probe (or undefined when no probe
     * runs) so callers — primarily integration tests — can await the
     * full chain.
     */
    async handleSSEError() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }

        if (this.token && !this.sseProbedSinceLastOpen) {
            this.sseProbedSinceLastOpen = true;
            const authFailed = await this.probeAuthAfterSSEFailure();
            if (authFailed) {
                this.preservePendingChanges();
                this.clearToken();
                console.info('🔒 Session ended — please log in again');
                this.notifyAuthRequired();
                return;
            }
        }

        this.reconnectAttempts++;

        // Check if we've exceeded max reconnection attempts
        if (this.reconnectAttempts > this.maxReconnectAttempts) {
            console.log(`❌ SSE connection failed after ${this.maxReconnectAttempts} attempts, switching to local development mode`);
            this.enableLocalDevMode();
            return;
        }

        // Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s (max)
        const delay = Math.min(
            1000 * Math.pow(2, this.reconnectAttempts - 1),
            this.maxReconnectDelay
        );

        console.log(`⏰ SSE reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);

        setTimeout(() => {
            console.log(`🔄 SSE reconnection attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts}`);
            this.connectSSE();
        }, delay);
    }

    /**
     * PWA Auth Step B5: send a tiny zero-write POST to /mcp to learn
     * whether the bridge will accept the current token. The bridge's
     * Step A2 resolveAuth runs before any storage access, so an empty
     * `data_update` returns 401 if the token is bad and 200 otherwise
     * — the perfect trial-balloon for distinguishing "token expired"
     * from "WiFi blip" after an SSE drop. EventSource onerror does not
     * expose the underlying HTTP status code, hence this companion
     * fetch.
     *
     * Returns true when the probe response is 401 (auth failed),
     * false otherwise (200, other status, or network error). Network
     * failures are treated as inconclusive — the caller continues with
     * normal SSE backoff so a transient outage doesn't bounce the
     * user back to the login screen.
     */
    async probeAuthAfterSSEFailure() {
        try {
            const response = await fetch('/mcp', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-AmorphDB-Device': this.deviceId,
                    'X-AmorphDB-Token': this.token,
                },
                body: JSON.stringify({
                    type: 'data_update',
                    user_id: this.getUserId(),
                    changes: {}
                })
            });
            return response.status === 401;
        } catch (e) {
            // Network error — can't determine auth status. Treat as
            // transient and let the normal backoff retry the SSE.
            return false;
        }
    }

    /**
     * Enable local development mode when SSE connection fails (Step 13: Local Development Mode)
     */
    enableLocalDevMode() {
        this.localDevMode = true;
        console.log('📱 AmorphDB PWA: Local development mode — no server connection');

        // Re-render with placeholder values visible
        const container = document.getElementById('app');
        if (container && this.sectionTree && this.dataTree) {
            // Store reference to this PWA's container for scoped queries
            this.container = container;
            container.innerHTML = '';
            const rootElement = this.renderSection(this.sectionTree, [], this.dataTree);
            container.appendChild(rootElement);
            this.wireInputHandlers(container, []);
        }
    }

    /**
     * Force local development mode immediately (for testing)
     */
    forceLocalDevMode() {
        this.reconnectAttempts = this.maxReconnectAttempts + 1;
        this.enableLocalDevMode();
    }

    /**
     * Simulate a server update for testing purposes
     * @param {string} path - Path to update
     * @param {any} value - Value to set
     */
    simulateServerUpdate(path, value) {
        console.log(`🧪 Simulating server update: ${path} = ${JSON.stringify(value)}`);
        this.applyServerUpdate(path, value);
    }

    /**
     * Fetch and parse app.html from the server or use inline content
     * @param {string} url - URL to fetch app.html from (optional)
     * @returns {Promise<Object>} Promise resolving to the parsed section tree
     */
    async loadApp(url = 'app.html') {
        console.log(`Starting loadApp() with URL: ${url}`);
        try {
            console.log('Attempting to fetch app.html...');
            const response = await fetch(url);
            console.log('Fetch response:', response.status, response.statusText);

            if (!response.ok) {
                throw new Error(`Failed to fetch ${url}: ${response.status} ${response.statusText}`);
            }

            console.log('Reading response text...');
            const appHtml = await response.text();
            console.log('App HTML loaded, length:', appHtml.length);
            console.log('First 200 chars of app.html:', appHtml.substring(0, 200));

            return this.parseApp(appHtml);
        } catch (error) {
            console.error('Error loading app.html:', error);
            console.log('Fetch failed, looking for inline app content...');

            // For local development, look for inline app content
            const appElement = document.getElementById('app-content');
            if (appElement) {
                console.log('Using inline app content for local development');
                return this.parseApp(appElement.innerHTML);
            }

            console.error('No inline app content found either, throwing error');
            throw error;
        }
    }

    /**
     * Render the section tree into the DOM
     * @param {string} containerId - ID of the container element to render into
     */
    renderDOM(containerId = 'app') {
        console.log('Starting DOM rendering...');

        const container = document.getElementById(containerId);
        if (!container) {
            console.error(`Container element with ID '${containerId}' not found`);
            return;
        }

        // Store reference to this PWA's app container for scoped queries
        this.appContainer = container;

        // Clear the container
        container.innerHTML = '';

        if (!this.sectionTree) {
            console.error('No section tree available for rendering');
            return;
        }

        // Render the root section
        const rootElement = this.renderSection(this.sectionTree, [], this.dataTree);
        container.appendChild(rootElement);

        console.log('DOM rendering complete');
    }

    /**
     * Render a single section and its children recursively
     * @param {Object} section - The section object from the section tree
     * @param {Array} path - Current path for data binding
     * @param {Object} dataNode - Corresponding data tree node
     * @returns {Element} The rendered DOM element
     */
    renderSection(section, path = [], dataNode = {}) {
        const pathArray = Array.isArray(path) ? path : (typeof path === 'string' ? path.split('.') : []);
        console.log(`renderSection: section="${section.name}" path=[${pathArray.join(',')}]`);

        // Create container div with CSS class based on section name
        const div = document.createElement('div');
        div.className = `section-${section.name}`;

        // Apply layout styling
        if (section.layout === 'horizontal') {
            div.style.display = 'flex';
            div.style.flexDirection = 'row';
        } else {
            div.style.display = 'flex';
            div.style.flexDirection = 'column';
        }

        // Handle list sections - store template and render items
        if (section.type === 'list') {
            console.log(`Rendering list section: ${section.name}`);
            div.classList.add('list-container');

            // Store the template for this list - path already includes section name
            const listPath = pathArray.join('.');
            console.log(`Storing list template and container for path: ${listPath}`);
            this.listTemplates.set(listPath, section.template);
            this.listContainers.set(listPath, div);

            // Step 11: List Windowing - add scroll detection for pagination
            this.setupListScrollDetection(div, listPath, section.load_max || 200);

            // Render existing list items if data is available
            const listData = dataNode;
            if (Array.isArray(listData) && listData.length > 0) {
                this.renderListContents(listPath, listData);
            }

            return div;
        }

        // Process template content
        if (section.template && section.template.trim()) {
            const processedTemplate = this.processTemplate(section.template, pathArray, dataNode);
            div.innerHTML = processedTemplate;
        }

        // Wire input event handlers
        this.wireInputHandlers(div, pathArray);

        // Recursively render child sections
        if (section.children && Object.keys(section.children).length > 0) {
            Object.keys(section.children).forEach(childName => {
                const childSection = section.children[childName];
                const childPath = [...pathArray, childName];
                const childDataNode = dataNode[childName] || {};

                const childElement = this.renderSection(childSection, childPath, childDataNode);
                div.appendChild(childElement);
            });
        }

        return div;
    }

    /**
     * Process template content, replacing {fieldname} placeholders with data-bound elements
     * @param {string} template - The template HTML string
     * @param {Array} path - Current section path
     * @param {Object} dataNode - Data for this section
     * @returns {string} Processed HTML with data-bind attributes
     */
    processTemplate(template, path, dataNode) {
        // Replace {fieldname} placeholders with <span data-bind="path"> elements
        // BUT for input value attributes, use appropriate placeholder values
        return template.replace(/\{([a-zA-Z_][a-zA-Z0-9_]*)\}/g, (match, fieldName, offset, string) => {
            // Step 13: In local dev mode, show the literal {fieldname} text so developers can see what data will go where
            if (this.localDevMode) {
                // Check if this {fieldname} is inside an input value attribute
                const beforeMatch = string.substring(0, offset);
                const afterMatch = string.substring(offset + match.length);
                const inInputValue = /value\s*=\s*["'][^"']*$/.test(beforeMatch) &&
                    /^[^"']*["']/.test(afterMatch);

                if (inInputValue) {
                    return match; // Keep {fieldname} in input values for placeholder text
                }

                // For spans and other elements, show the placeholder as visible text
                const pathArray = Array.isArray(path) ? path : (typeof path === 'string' ? path.split('.') : []);
                const cleanedPath = pathArray
                    .filter(p => p != null && typeof p === 'string' && p.trim() !== '')
                    .map(p => p.trim());
                const pathParts = [...cleanedPath, fieldName.trim()];
                const fullPath = pathParts.join('.').replace(/^\.+|\.+$/g, '');

                return `<span data-bind="${fullPath}">${match}</span>`;
            }

            // Normal mode: Check if this {fieldname} is inside an input value attribute
            const beforeMatch = string.substring(0, offset);
            const afterMatch = string.substring(offset + match.length);

            // Simple check: if we're inside value="..." of an input, use appropriate placeholder
            const inInputValue = /value\s*=\s*["'][^"']*$/.test(beforeMatch) &&
                                 /^[^"']*["']/.test(afterMatch);

            if (inInputValue) {
                // Check if this is a number input by looking backwards for type="number"
                const inputTypeMatch = beforeMatch.match(/<input[^>]*type\s*=\s*["']number["'][^>]*value\s*=\s*["'][^"']*$/);
                if (inputTypeMatch) {
                    return '0'; // Use 0 for number inputs to prevent browser parsing warnings
                } else {
                    return ''; // Use empty string for other input types
                }
            }

            // Normal span replacement for text content
            const pathArray = Array.isArray(path) ? path : (typeof path === 'string' ? path.split('.') : []);
            const cleanedPath = pathArray
                .filter(p => p != null && typeof p === 'string' && p.trim() !== '')
                .map(p => p.trim());
            const pathParts = [...cleanedPath, fieldName.trim()];
            const fullPath = pathParts.join('.').replace(/^\.+|\.+$/g, '');

            const value = dataNode[fieldName] || '';
            return `<span data-bind="${fullPath}">${this.escapeHtml(value)}</span>`;
        });
    }

    /**
     * Wire event handlers for input elements with data-bind attributes
     * @param {Element} container - Container element to search within
     * @param {Array} path - Current section path for data binding
     */
    wireInputHandlers(container, path) {
        // Find all input elements
        const inputs = container.querySelectorAll('input, select, textarea');

        inputs.forEach(input => {
            // Check if the input has a {fieldname} in its value attribute
            const valueAttr = input.getAttribute('value');
            if (valueAttr && valueAttr.includes('{') && valueAttr.includes('}')) {
                // Extract field name from {fieldname}
                const fieldMatch = valueAttr.match(/\{([a-zA-Z_][a-zA-Z0-9_]*)\}/);
                if (fieldMatch) {
                    const fieldName = fieldMatch[1];

                    // Aggressive path cleaning
                    const pathArray = Array.isArray(path) ? path : (typeof path === 'string' ? path.split('.') : []);
                    const cleanedPath = pathArray
                        .filter(p => p != null && typeof p === 'string' && p.trim() !== '')
                        .map(p => p.trim());
                    const pathParts = [...cleanedPath, fieldName.trim()];
                    const fullPath = pathParts.join('.').replace(/^\.+|\.+$/g, '');

                    // Set data-bind attribute
                    input.setAttribute('data-bind', fullPath);

                    // Remove the {fieldname} from value and set actual value
                    const actualValue = this.getValueAtPath(fullPath);

                    // Set appropriate value based on input type
                    if (input.type === 'number') {
                        input.value = actualValue || 0;
                    } else if (input.type === 'checkbox') {
                        input.checked = !!actualValue;
                    } else {
                        input.value = actualValue || '';
                    }

                    // Remove the value attribute to prevent conflicts
                    input.removeAttribute('value');

                    // Wire change and blur event handlers
                    const handler = () => {
                        this.updateDataFromInput(fullPath, input.value, input.type);
                    };

                    input.addEventListener('change', handler);
                    input.addEventListener('blur', handler);
                }
            }
        });
    }

    /**
     * Get a value from the data tree by path
     * @param {string} path - Dot-separated path to the value
     * @returns {any} The value at the path, or undefined if not found
     */
    getValueAtPath(path) {
        const parts = path.split('.');
        let current = this.dataTree;

        for (const part of parts) {
            if (current && typeof current === 'object' && part in current) {
                current = current[part];
            } else {
                return undefined;
            }
        }

        return current;
    }

    /**
     * Update the data tree from input field changes
     * @param {string} path - Dot-separated path to update
     * @param {string} value - New value from input field
     * @param {string} inputType - Type of the input field
     */
    updateDataFromInput(path, value, inputType) {
        console.log(`Input change: ${path} = ${value} (type: ${inputType})`);

        // Convert value based on input type
        let convertedValue = value;
        if (inputType === 'number') {
            convertedValue = parseFloat(value) || 0;
        } else if (inputType === 'checkbox') {
            convertedValue = value === 'on' || value === true;
        }

        // Update via the proxied data (this will trigger change tracking)
        this.setValueAtPath(this.proxiedData, path, convertedValue);
    }

    /**
     * Set a value in an object by path
     * @param {Object} obj - The object to update
     * @param {string} path - Dot-separated path
     * @param {any} value - Value to set
     */
    setValueAtPath(obj, path, value) {
        const parts = path.split('.');
        let current = obj;

        for (let i = 0; i < parts.length - 1; i++) {
            const part = parts[i];
            if (!(part in current) || typeof current[part] !== 'object') {
                current[part] = {};
            }
            current = current[part];
        }

        current[parts[parts.length - 1]] = value;
    }

    /**
     * Update all DOM elements bound to a specific data path
     * @param {string} path - Dot-separated path to the field
     * @param {any} value - New value to set
     */
    updateField(path, value) {
        // Step 12: Handle _shown properties for section visibility
        if (path.endsWith('._shown')) {
            this.updateSectionVisibility(path, value);
            return;
        }

        // Find all elements with matching data-bind attribute
        const elements = document.querySelectorAll(`[data-bind="${path}"]`);

        if (elements.length === 0) {
            console.warn(`No elements found for data-bind="${path}"`);
            return;
        }

        console.log(`Updating ${elements.length} DOM element(s) for ${path} = ${value}`);

        elements.forEach(element => {
            const tagName = element.tagName.toLowerCase();
            const inputType = element.type ? element.type.toLowerCase() : '';

            if (tagName === 'input' || tagName === 'textarea') {
                if (inputType === 'checkbox') {
                    element.checked = !!value;
                } else {
                    element.value = value || '';
                }
            } else if (tagName === 'select') {
                element.value = value || '';
            } else {
                // For all other elements (spans, divs, etc.), update textContent
                element.textContent = value || '';
            }
        });
    }

    /**
     * Update section visibility based on _shown property (Step 12: Shown/Hidden)
     * @param {string} path - The path ending with ._shown
     * @param {boolean} value - Whether the section should be shown
     */
    updateSectionVisibility(path, value) {
        // Extract section path by removing ._shown suffix
        const sectionPath = path.replace(/\._shown$/, '');
        const sectionName = sectionPath.split('.').pop(); // Get the last part of the path

        console.log(`🔧 Updating section visibility: ${sectionPath} (${sectionName}) => ${value ? 'shown' : 'hidden'}`);

        // Find the section container element within this PWA's app container, not entire document
        const sectionElement = this.appContainer ?
            this.appContainer.querySelector(`.section-${sectionName}`) :
            document.querySelector(`.section-${sectionName}`);

        if (!sectionElement) {
            console.warn(`⚠️  Section element not found: .section-${sectionName} within PWA app container`);
            return;
        }

        // Toggle visibility using display CSS property
        if (value) {
            // Show the section - restore original display style
            sectionElement.style.display = sectionElement.dataset.originalDisplay || 'flex';
            console.log(`✅ Section ${sectionName} shown`);
        } else {
            // Hide the section - store original display style and hide
            if (!sectionElement.dataset.originalDisplay) {
                sectionElement.dataset.originalDisplay = sectionElement.style.display || 'flex';
            }
            sectionElement.style.display = 'none';
            console.log(`🙈 Section ${sectionName} hidden`);
        }
    }

    /**
     * Register a watcher callback for a data path (Step 9: Local Watchers)
     * @param {string} path - The data path to watch (e.g., "banner.app_name")
     * @param {Function} callback - Function to call when the path or its children change
     */
    watch(path, callback) {
        if (!this.watchers.has(path)) {
            this.watchers.set(path, []);
        }
        this.watchers.get(path).push(callback);
        console.log(`👁️ Watcher registered for path: ${path}`);
    }

    /**
     * Notify watchers when a data path changes (Step 9: Local Watchers)
     * Fires callbacks for exact matches AND parent paths
     * @param {string} path - The path that changed
     * @param {any} value - The new value
     */
    notifyWatchers(path, value) {
        console.log(`📢 Notifying watchers for path: ${path}`);

        // Fire watchers for exact path match
        if (this.watchers.has(path)) {
            const callbacks = this.watchers.get(path);
            console.log(`🔥 Firing ${callbacks.length} watcher(s) for exact match: ${path}`);
            callbacks.forEach(callback => {
                try {
                    callback(value, path);
                } catch (error) {
                    console.error(`❌ Error in watcher for ${path}:`, error);
                }
            });
        }

        // Fire watchers for parent paths
        // For example, if "orders.0.product" changes, fire watchers for "orders" and "orders.0"
        const pathParts = path.split('.');
        for (let i = pathParts.length - 1; i > 0; i--) {
            const parentPath = pathParts.slice(0, i).join('.');
            if (this.watchers.has(parentPath)) {
                const callbacks = this.watchers.get(parentPath);
                console.log(`🔥 Firing ${callbacks.length} parent watcher(s) for: ${parentPath}`);
                callbacks.forEach(callback => {
                    try {
                        // For parent watchers, pass the child path as second parameter
                        callback(this.getValueAtPath(parentPath), parentPath, path);
                    } catch (error) {
                        console.error(`❌ Error in parent watcher for ${parentPath}:`, error);
                    }
                });
            }
        }
    }

    /**
     * Register a path to receive SSE updates regardless of current view (Step 12: Focused Updates)
     * @param {string} path - The data path to keep alive (e.g., "notifications", "unread_count")
     */
    keepLive(path) {
        this.keepLivePaths.add(path);
        console.log(`📌 Registered keepLive path: ${path}`);
        console.log(`📌 Active keepLive paths: ${Array.from(this.keepLivePaths).join(', ')}`);
    }

    /**
     * Check if a path should receive updates based on focused update rules
     * @param {string} path - The path to check
     * @returns {boolean} True if the path should receive updates
     */
    shouldReceiveUpdates(path) {
        // Always receive updates for keepLive paths
        if (this.keepLivePaths.has(path)) {
            return true;
        }

        // Check if any parent path is in keepLive
        const pathParts = path.split('.');
        for (let i = pathParts.length - 1; i > 0; i--) {
            const parentPath = pathParts.slice(0, i).join('.');
            if (this.keepLivePaths.has(parentPath)) {
                return true;
            }
        }

        // TODO: In full implementation, check if current view matches
        // For now, allow all updates (default behavior)
        return true;
    }

    /**
     * Escape HTML to prevent XSS
     * @param {string} str - String to escape
     * @returns {string} Escaped string
     */
    escapeHtml(str) {
        if (typeof str !== 'string') return str;

        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    /**
     * Get nested value from object using dot-notation path
     * @param {Object} obj - Object to traverse
     * @param {string} path - Dot-notation path (e.g., "workspace.main.orders")
     * @returns {any} The value at the path, or undefined if not found
     */
    getNestedValue(obj, path) {
        return path.split('.').reduce((current, key) => {
            return current && current[key] !== undefined ? current[key] : undefined;
        }, obj);
    }

    /**
     * Render the complete contents of a list section
     * @param {string} listPath - Path to the list in the data tree
     * @param {Array} listData - Array of list item data
     */
    renderListContents(listPath, listData) {
        console.log(`Rendering list contents for ${listPath}, ${listData.length} items`);

        const container = this.listContainers.get(listPath);
        const template = this.listTemplates.get(listPath);

        if (!container || !template) {
            console.warn(`Missing container or template for list: ${listPath}`);
            return;
        }

        // Clear existing content
        container.innerHTML = '';

        // Render each item
        for (let i = 0; i < listData.length; i++) {
            const itemElement = this.createListItem(template, listPath, i, listData[i]);
            container.appendChild(itemElement);
        }
    }

    /**
     * Add a single list item to the DOM
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} index - Index of the new item
     * @param {Object} itemData - Data for the new item
     */
    addListItem(listPath, index, itemData) {
        console.log(`Adding list item at ${listPath}.${index}:`, itemData);

        const container = this.listContainers.get(listPath);
        const template = this.listTemplates.get(listPath);

        if (!container || !template) {
            console.warn(`Missing container or template for list: ${listPath}`);
            return;
        }

        const itemElement = this.createListItem(template, listPath, index, itemData);
        container.appendChild(itemElement);
    }

    /**
     * Remove a list item from the DOM and reindex remaining items
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} removedIndex - Index of the item to remove
     */
    removeListItem(listPath, removedIndex) {
        console.log(`Removing list item at ${listPath}.${removedIndex}`);

        const container = this.listContainers.get(listPath);
        if (!container) {
            console.warn(`Missing container for list: ${listPath}`);
            return;
        }

        const children = Array.from(container.children);
        if (removedIndex < children.length) {
            // Remove the item element
            container.removeChild(children[removedIndex]);

            // Reindex remaining items
            this.reindexListItems(listPath, removedIndex);
        }
    }

    /**
     * Update data-bind attributes for list items after an item is removed
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} startIndex - Index to start reindexing from
     */
    reindexListItems(listPath, startIndex) {
        const container = this.listContainers.get(listPath);
        if (!container) return;

        const children = Array.from(container.children);
        for (let i = startIndex; i < children.length; i++) {
            const itemElement = children[i];
            this.updateListItemBindings(itemElement, listPath, i, i + 1); // old index = i + 1
        }
    }

    /**
     * Update data-bind attributes in a list item element
     * @param {Element} itemElement - The list item DOM element
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} newIndex - New index for the item
     * @param {number} oldIndex - Old index for the item (for replacement)
     */
    updateListItemBindings(itemElement, listPath, newIndex, oldIndex) {
        const boundElements = itemElement.querySelectorAll('[data-bind]');
        boundElements.forEach(element => {
            const currentBind = element.getAttribute('data-bind');
            // Replace old index with new index in the data-bind path
            const oldPath = `${listPath}.${oldIndex}.`;
            const newPath = `${listPath}.${newIndex}.`;
            if (currentBind.startsWith(oldPath)) {
                const newBind = currentBind.replace(oldPath, newPath);
                element.setAttribute('data-bind', newBind);
            }
        });
    }

    /**
     * Create a DOM element for a single list item
     * @param {string} template - HTML template for list items
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} index - Index of this item in the list
     * @param {Object} itemData - Data for this item
     * @returns {Element} The created list item element
     */
    createListItem(template, listPath, index, itemData) {
        // Create a temporary container to parse the template
        const tempDiv = document.createElement('div');

        // Process template with item data and index-specific paths
        const processedTemplate = this.processListTemplate(template, listPath, index, itemData);
        tempDiv.innerHTML = processedTemplate;

        // Extract the first child (the actual list item)
        const itemElement = tempDiv.firstElementChild;

        // Wire event handlers for any input elements in the item
        this.wireInputHandlers(itemElement, listPath.split('.').concat([index.toString()]));

        return itemElement;
    }

    /**
     * Process a list item template, replacing {fieldname} with data-bound elements
     * @param {string} template - The template HTML string
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} index - Index of this item
     * @param {Object} itemData - Data for this item
     * @returns {string} Processed HTML with data-bind attributes
     */
    processListTemplate(template, listPath, index, itemData) {
        return template.replace(/\{([a-zA-Z_][a-zA-Z0-9_]*)\}/g, (match, fieldName, offset, string) => {
            // Check if this {fieldname} is inside an input value attribute
            const beforeMatch = string.substring(0, offset);
            const afterMatch = string.substring(offset + match.length);

            const inInputValue = /value\s*=\s*["'][^"']*$/.test(beforeMatch) &&
                                 /^[^"']*["']/.test(afterMatch);

            if (inInputValue) {
                return match; // Leave {fieldname} as-is for input values
            }

            // Create data-bind path with list index
            const fullPath = `${listPath}.${index}.${fieldName}`;
            const value = itemData[fieldName] || '';

            return `<span data-bind="${fullPath}">${this.escapeHtml(value)}</span>`;
        });
    }

    // ===== Step 11: List Windowing Methods =====

    /**
     * Setup scroll detection for list windowing
     * @param {Element} container - The list container DOM element
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} loadMax - Maximum items to load at once
     */
    setupListScrollDetection(container, listPath, loadMax) {
        console.log(`Setting up scroll detection for list: ${listPath}, load_max: ${loadMax}`);

        let scrollTimeout;
        container.addEventListener('scroll', () => {
            // Debounce scroll events
            if (scrollTimeout) clearTimeout(scrollTimeout);
            scrollTimeout = setTimeout(() => {
                this.checkListScrollPosition(container, listPath, loadMax);
            }, 150);
        });
    }

    /**
     * Check if we need to load more items based on scroll position
     * @param {Element} container - The list container DOM element
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} loadMax - Maximum items to load at once
     */
    checkListScrollPosition(container, listPath, loadMax) {
        const scrollTop = container.scrollTop;
        const scrollHeight = container.scrollHeight;
        const clientHeight = container.clientHeight;

        // Calculate how close we are to the bottom (80% threshold)
        const scrollPercent = (scrollTop + clientHeight) / scrollHeight;

        if (scrollPercent > 0.8) {
            console.log(`Near bottom of list ${listPath}, checking if more items needed...`);
            this.requestNextListPage(listPath, loadMax);
        }
    }

    /**
     * Request the next page of items for a list
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} loadMax - Maximum items to load at once
     */
    async requestNextListPage(listPath, loadMax) {
        const listData = this.getNestedValue(this.rawData, listPath);
        if (!Array.isArray(listData)) {
            console.warn(`Cannot request next page - ${listPath} is not a list`);
            return;
        }

        const currentThrough = listData._through || 0;
        const totalItems = listData._total || 0;

        // Don't request if we already have all items
        if (currentThrough >= totalItems) {
            console.log(`All items loaded for ${listPath} (${currentThrough}/${totalItems})`);
            return;
        }

        const nextFrom = currentThrough + 1;
        const requestCount = Math.min(loadMax, totalItems - currentThrough);

        console.log(`Requesting next page for ${listPath}: from ${nextFrom}, count ${requestCount}`);

        // Step 11: Send MCP message for list page request
        const mcpMessage = {
            type: 'list_page',
            user_id: this.getUserId(),
            path: listPath,
            from: nextFrom,
            count: requestCount
        };

        // In local dev mode or for testing, simulate the request
        if (this.localDevMode) {
            console.log('📱 Local dev mode: Simulating list page request', mcpMessage);
            this.simulateListPageResponse(listPath, nextFrom, requestCount);
            return;
        }

        try {
            // PWA Auth Step B2: Authenticated list paging carries the same
            // device + token headers as flushChanges so the bridge can
            // resolve the user identity and serve from
            // world.apps.<app>.users.<identity>.*.
            const headers = {
                'Content-Type': 'application/json',
                'X-AmorphDB-Device': this.deviceId,
            };
            if (this.token) {
                headers['X-AmorphDB-Token'] = this.token;
            }

            const response = await fetch('/mcp', {
                method: 'POST',
                headers: headers,
                body: JSON.stringify(mcpMessage)
            });

            if (response.ok) {
                console.log(`✅ List page request sent for ${listPath}`);
            } else if (response.status === 401) {
                // PWA Auth Step B5: list paging has no in-flight changes
                // to preserve (it's a read-only request), so we don't
                // touch pendingChanges — but the token is gone, so we
                // route through clearToken + notifyAuthRequired the same
                // way flushChanges does, with the same friendly message.
                if (this.token) {
                    this.clearToken();
                }
                console.info('🔒 Session ended — please log in again');
                this.notifyAuthRequired();
            } else {
                console.error(`❌ List page request failed for ${listPath}:`, response.statusText);
            }
        } catch (error) {
            console.error(`❌ List page request error for ${listPath}:`, error);
        }
    }

    /**
     * Handle incoming list page data from server
     * @param {string} listPath - Path to the list in the data tree
     * @param {Array} items - New items to append
     * @param {number} from - Starting index of new items
     * @param {number} total - Total number of items available
     */
    handleListPageUpdate(listPath, items, from, total) {
        console.log(`Handling list page update for ${listPath}: ${items.length} items from ${from}, total ${total}`);

        const listData = this.getNestedValue(this.rawData, listPath);
        if (!Array.isArray(listData)) {
            console.warn(`Cannot update - ${listPath} is not a list`);
            return;
        }

        // Update windowing properties
        const currentLength = listData.length;
        listData._from = listData._from || 0;
        listData._through = from + items.length - 1;
        listData._total = total;

        // Append new items to the list
        items.forEach(item => {
            listData.push(item);
        });

        // Check if we should evict items from far end (3x load_max limit)
        const loadMax = listData._load_max || 200;
        const maxWindow = loadMax * 3;

        if (listData.length > maxWindow) {
            const itemsToEvict = listData.length - maxWindow;
            console.log(`Evicting ${itemsToEvict} items from start of ${listPath} to maintain window size`);

            // Remove items from beginning
            listData.splice(0, itemsToEvict);
            listData._from += itemsToEvict;

            // Re-render entire list to update indices
            this.renderListContents(listPath, listData);
        } else {
            // Just append new items to DOM
            for (let i = 0; i < items.length; i++) {
                const itemIndex = currentLength + i;
                this.addListItem(listPath, itemIndex, items[i]);
            }
        }

        console.log(`List ${listPath} now has ${listData.length} items loaded (${listData._from}-${listData._through} of ${listData._total})`);
    }

    /**
     * Simulate list page response for testing
     * @param {string} listPath - Path to the list in the data tree
     * @param {number} from - Starting index
     * @param {number} count - Number of items to generate
     */
    simulateListPageResponse(listPath, from, count) {
        console.log(`Simulating list page response for ${listPath}: ${count} items from ${from}`);

        // Generate dummy items
        const items = [];
        for (let i = 0; i < count; i++) {
            const itemIndex = from + i;
            items.push({
                product: `Product ${itemIndex}`,
                quantity: Math.floor(Math.random() * 10) + 1,
                amount: `$${((Math.random() * 100) + 10).toFixed(2)}`,
                status: ['pending', 'processing', 'shipped', 'delivered'][Math.floor(Math.random() * 4)]
            });
        }

        // Simulate total of 1000 items for testing
        const totalItems = 1000;

        // Apply the update
        setTimeout(() => {
            this.handleListPageUpdate(listPath, items, from, totalItems);
        }, 100); // Small delay to simulate network
    }

    /**
     * Setup form submission handler for testing purposes
     * Watches for submit button presses and adds form data to orders list
     */
    setupFormSubmissionHandler() {
        console.log('Setting up form submission handler for testing...');

        // Create a simple watcher for the submit button
        let lastSubmitValue = this.proxiedData.workspace?.main?.order_form?.submit || false;

        // Check for submit button changes every 100ms
        const checkSubmit = () => {
            const currentSubmitValue = this.proxiedData.workspace?.main?.order_form?.submit || false;

            if (currentSubmitValue === 'down' && lastSubmitValue !== 'down') {
                console.log('Form submission detected, processing order...');

                // Get form data
                const orderForm = this.proxiedData.workspace.main.order_form;
                const product = orderForm.product || '';
                const quantity = orderForm.quantity || 0;

                if (product.trim() && quantity > 0) {
                    // Create order item
                    const orderItem = {
                        product: product,
                        quantity: quantity,
                        amount: `$${(quantity * 9.99).toFixed(2)}`, // Simple pricing
                        status: 'pending'
                    };

                    // Add to orders list
                    this.proxiedData.workspace.main.orders.push(orderItem);
                    console.log('Order added to list:', orderItem);

                    // Clear form
                    orderForm.product = '';
                    orderForm.quantity = 0;

                    // Reset submit button
                    orderForm.submit = false;
                } else {
                    console.warn('Form submission ignored - missing product or quantity');
                    orderForm.submit = false;
                }
            }

            lastSubmitValue = currentSubmitValue;
        };

        // Start checking periodically
        setInterval(checkSubmit, 100);
    }

    /**
     * Initialize deep link mapping (Step 10: Deep Link Mapper)
     * Implements URL ↔ data synchronization
     */
    initializeDeepLinking() {
        console.log('🔗 Setting up deep link mapper...');

        // Ensure navigation object exists in data tree
        if (!this.proxiedData.navigation) {
            console.log('📁 Creating navigation object in data tree');
            this.rawData.navigation = {};
            this.proxiedData.navigation = this.createProxy(this.rawData.navigation, 'navigation');
        }

        // Step 1: On page load - read window.location.pathname, set data.navigation.current
        const currentPath = window.location.pathname;
        console.log(`🌐 Initial URL path: ${currentPath}`);

        // Set initial navigation state without triggering URL update
        this.rawData.navigation.current = currentPath;
        this.updateField('navigation.current', currentPath);
        this.notifyWatchers('navigation.current', currentPath);

        console.log(`📍 Set data.navigation.current = "${currentPath}"`);

        // Step 2: On popstate event (browser back/forward) - update data.navigation.current
        window.addEventListener('popstate', (event) => {
            const newPath = window.location.pathname;
            console.log(`⬅️ Popstate event - URL changed to: ${newPath}`);

            // Update data without triggering watcher that would push URL
            this.rawData.navigation.current = newPath;
            this.updateField('navigation.current', newPath);
            this.notifyWatchers('navigation.current', newPath);

            console.log(`📍 Updated data.navigation.current = "${newPath}" via popstate`);
        });

        // Step 3: Watch navigation.current - push URL via history.pushState
        // Step 4: Prevent infinite loops - don't push if URL already matches
        this.watch('navigation.current', (value) => {
            console.log(`🔍 Navigation watcher fired: navigation.current = "${value}"`);

            // Check if URL already matches to prevent infinite loop
            if (window.location.pathname !== value) {
                console.log(`🚀 Pushing new URL: ${value} (current: ${window.location.pathname})`);
                history.pushState(null, '', value);
                console.log(`✅ URL pushed successfully to: ${value}`);
            } else {
                console.log(`⏸️  URL already matches "${value}" - skipping push to prevent loop`);
            }
        });

        console.log('✅ Deep link mapper initialized successfully');
    }

    /**
     * Initialize the PWA - main entry point
     */
    async init() {
        console.log('Starting AmorphDB PWA initialization...');
        try {
            const result = await this.loadApp();
            console.log('loadApp() completed, result:', result);
            console.log('sectionTree set to:', this.sectionTree);

            // Generate the data tree from the section tree
            console.log('Generating data tree...');
            this.generateDataTree();
            console.log('dataTree set to:', this.dataTree);

            // Create proxy wrapper for change tracking (Step 3)
            console.log('Creating proxy wrapper for change tracking...');

            // Create deep copy for proxy to avoid modifying original data
            const dataForProxy = JSON.parse(JSON.stringify(this.dataTree));
            this.rawData = this.dataTree;  // Store original unmodified data
            this.proxiedData = this.createProxy(dataForProxy);
            console.log('Proxy created, proxiedData available for change tracking');

            // Expose proxied data as global 'data' variable for onclick handlers
            window.data = this.proxiedData;
            console.log('Global data variable exposed for onclick handlers');

            // Export watch function globally for use in app.js (Step 9: Local Watchers)
            window.watch = (path, callback) => this.watch(path, callback);
            console.log('Global watch function exposed for custom watchers');

            // Export keepLive function globally for use in app.js (Step 12: Focused Updates)
            window.keepLive = (path) => this.keepLive(path);
            console.log('Global keepLive function exposed for focused updates');

            // Add a watcher for form submission (for testing purposes)
            this.setupFormSubmissionHandler();

            // Render the DOM (Step 4)
            console.log('Rendering DOM from section tree...');
            this.renderDOM();
            console.log('DOM rendering complete');

            // PWA Auth Step B3: Once the DOM is rendered, apply pre-auth or
            // post-auth visibility to top-level sections. With no token in
            // localStorage, only `login` and `signup` are visible; with a
            // token already persisted from a prior session, those auth
            // sections stay hidden and the rest of the app is shown. Apps
            // with no login/signup sections (guest mode) skip this entirely.
            this.applyAuthVisibility();
            console.log('Auth visibility applied');

            // Initialize deep linking (Step 10: Deep Link Mapper)
            console.log('Initializing deep link mapper...');
            this.initializeDeepLinking();
            console.log('Deep link mapper initialized');

            // Start SSE connection for real-time updates (Step 8)
            console.log('Establishing SSE connection...');
            this.connectSSE();

            console.log('AmorphDB PWA initialized successfully');

            // Fire custom event to notify test code
            const event = new CustomEvent('amorphPWAReady', {
                detail: {
                    sectionTree: this.sectionTree,
                    dataTree: this.dataTree,
                    proxiedData: this.proxiedData,
                    changes: this.changes,
                    amorphPWA: this  // Provide access to instance for testing
                }
            });
            document.dispatchEvent(event);

        } catch (error) {
            console.error('Failed to initialize AmorphDB PWA:', error);

            // Fire error event
            const errorEvent = new CustomEvent('amorphPWAError', {
                detail: { error: error }
            });
            document.dispatchEvent(errorEvent);
        }
    }
}

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        window.amorphPWA = new AmorphPWA();
        window.amorphPWA.init();
    });
} else {
    window.amorphPWA = new AmorphPWA();
    window.amorphPWA.init();
}

// Export for testing
if (typeof module !== 'undefined' && module.exports) {
    module.exports = AmorphPWA;
}