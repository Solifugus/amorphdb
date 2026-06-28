#!/usr/bin/env node

// Simple script to check the expected data tree structure
const fs = require('fs');

// Test HTML structure from app.html
const appHtml = `<section name="app">
    <section name="banner" layout="horizontal">
        <h1>{app_name}</h1>
        <span class="user">{user_name}</span>
    </section>

    <section name="workspace" layout="horizontal">
        <section name="sidebar">
            <h3>Menu</h3>
            <section name="menu_items" type="list">
                <a onclick="data.navigation.current = '{target}'">{label}</a>
            </section>
        </section>

        <section name="main">
            <h2>{page_title}</h2>

            <section name="order_form">
                <label>Product</label>
                <input value="{product}">
                <label>Quantity</label>
                <input type="number" value="{quantity}">
                <button onclick="data.workspace.main.order_form.submit = 'down'">
                    Submit Order
                </button>
            </section>

            <section name="orders" type="list" show_max="20" load_max="100">
                <div class="order-row">
                    <span>{product}</span>
                    <span>{quantity}</span>
                    <span class="money">{amount}</span>
                    <span class="status">{status}</span>
                </div>
            </section>
        </section>
    </section>

    <section name="status_bar" layout="horizontal">
        <span>{status_message}</span>
        <span>v{version}</span>
    </section>
</section>`;

// Mock DOM and global environment
global.document = {
    createElement: () => ({
        style: {},
        dataset: {},
        className: '',
        innerHTML: '',
        appendChild: () => {},
        querySelectorAll: () => [],
        querySelector: () => null,
        setAttribute: () => {},
        getAttribute: () => null
    }),
    getElementById: () => null,
    querySelectorAll: () => [],
    querySelector: () => null
};

global.window = { location: { pathname: '/' } };
global.localStorage = { getItem: () => null, setItem: () => {} };
global.EventSource = function() { this.close = () => {}; };
global.queueMicrotask = (fn) => setTimeout(fn, 0);
global.fetch = () => Promise.resolve({ ok: true });
global.DOMParser = function() {
    this.parseFromString = function(htmlString, type) {
        // Simple mock - just return an object that can be queried
        const doc = {
            querySelectorAll: function(selector) {
                if (selector === 'section') {
                    // Return mock sections based on the HTML structure
                    const appSection = {
                        getAttribute: function(name) {
                            if (name === 'name') return 'app';
                            if (name === 'layout') return null;
                            if (name === 'type') return null;
                            if (name === 'show_max') return null;
                            if (name === 'load_max') return null;
                            return null;
                        },
                        cloneNode: () => ({
                            querySelectorAll: () => [
                                // banner section
                                {
                                    getAttribute: function(name) {
                                        if (name === 'name') return 'banner';
                                        if (name === 'layout') return 'horizontal';
                                        return null;
                                    },
                                    cloneNode: () => ({
                                        querySelectorAll: () => [],
                                        innerHTML: '<h1>{app_name}</h1><span class="user">{user_name}</span>',
                                        contains: () => false
                                    })
                                },
                                // workspace section
                                {
                                    getAttribute: function(name) {
                                        if (name === 'name') return 'workspace';
                                        if (name === 'layout') return 'horizontal';
                                        return null;
                                    },
                                    cloneNode: () => ({
                                        querySelectorAll: () => [
                                            // sidebar
                                            {
                                                getAttribute: function(name) {
                                                    if (name === 'name') return 'sidebar';
                                                    return null;
                                                },
                                                cloneNode: () => ({
                                                    querySelectorAll: () => [
                                                        // menu_items
                                                        {
                                                            getAttribute: function(name) {
                                                                if (name === 'name') return 'menu_items';
                                                                if (name === 'type') return 'list';
                                                                return null;
                                                            },
                                                            cloneNode: () => ({
                                                                querySelectorAll: () => [],
                                                                innerHTML: '<a onclick="data.navigation.current = \'{target}\'">{label}</a>',
                                                                contains: () => false
                                                            })
                                                        }
                                                    ],
                                                    innerHTML: '<h3>Menu</h3>',
                                                    contains: () => true,
                                                    removeChild: () => {}
                                                })
                                            },
                                            // main
                                            {
                                                getAttribute: function(name) {
                                                    if (name === 'name') return 'main';
                                                    return null;
                                                },
                                                cloneNode: () => ({
                                                    querySelectorAll: () => [
                                                        // order_form
                                                        {
                                                            getAttribute: function(name) {
                                                                if (name === 'name') return 'order_form';
                                                                return null;
                                                            },
                                                            cloneNode: () => ({
                                                                querySelectorAll: () => [],
                                                                innerHTML: '<label>Product</label><input value="{product}"><label>Quantity</label><input type="number" value="{quantity}"><button onclick="data.workspace.main.order_form.submit = \'down\'">Submit Order</button>',
                                                                contains: () => false
                                                            })
                                                        },
                                                        // orders
                                                        {
                                                            getAttribute: function(name) {
                                                                if (name === 'name') return 'orders';
                                                                if (name === 'type') return 'list';
                                                                if (name === 'show_max') return '20';
                                                                if (name === 'load_max') return '100';
                                                                return null;
                                                            },
                                                            cloneNode: () => ({
                                                                querySelectorAll: () => [],
                                                                innerHTML: '<div class="order-row"><span>{product}</span><span>{quantity}</span><span class="money">{amount}</span><span class="status">{status}</span></div>',
                                                                contains: () => false
                                                            })
                                                        }
                                                    ],
                                                    innerHTML: '<h2>{page_title}</h2>',
                                                    contains: () => true,
                                                    removeChild: () => {}
                                                })
                                            }
                                        ],
                                        innerHTML: '',
                                        contains: () => true,
                                        removeChild: () => {}
                                    })
                                },
                                // status_bar section
                                {
                                    getAttribute: function(name) {
                                        if (name === 'name') return 'status_bar';
                                        if (name === 'layout') return 'horizontal';
                                        return null;
                                    },
                                    cloneNode: () => ({
                                        querySelectorAll: () => [],
                                        innerHTML: '<span>{status_message}</span><span>v{version}</span>',
                                        contains: () => false
                                    })
                                }
                            ],
                            innerHTML: '',
                            contains: () => true,
                            removeChild: () => {}
                        })
                    };
                    return [appSection];
                }
                return [];
            }
        };
        return doc;
    };
};

// Now read and execute the PWA code safely
const pwaFile = fs.readFileSync('/home/solifugus/development/amorphdb/web/boilerplate/test/amorphdb-pwa.js', 'utf8');

// Extract just the class definition safely
let AmorphPWA;
try {
    eval(pwaFile);

    console.log('🧪 Testing Data Tree Structure...\n');

    // Create PWA instance and parse HTML
    const pwa = new AmorphPWA();
    const sectionTree = pwa.parseApp(appHtml);
    const dataTree = pwa.generateDataTree(sectionTree);

    console.log('📊 Section Tree Structure:');
    console.log(JSON.stringify(sectionTree, null, 2));

    console.log('\n📊 Data Tree Structure:');
    console.log(JSON.stringify(dataTree, null, 2));

    console.log('\n✅ Expected Field Locations:');
    console.log('  banner.app_name:', dataTree?.banner?.app_name !== undefined ? '✅' : '❌');
    console.log('  banner.user_name:', dataTree?.banner?.user_name !== undefined ? '✅' : '❌');
    console.log('  workspace.main.page_title:', dataTree?.workspace?.main?.page_title !== undefined ? '✅' : '❌');
    console.log('  workspace.main.order_form.product:', dataTree?.workspace?.main?.order_form?.product !== undefined ? '✅' : '❌');
    console.log('  workspace.main.order_form.quantity:', dataTree?.workspace?.main?.order_form?.quantity !== undefined ? '✅' : '❌');
    console.log('  workspace.main.orders (array):', Array.isArray(dataTree?.workspace?.main?.orders) ? '✅' : '❌');
    console.log('  workspace.sidebar._shown:', dataTree?.workspace?.sidebar?._shown !== undefined ? '✅' : '❌');
    console.log('  status_bar.status_message:', dataTree?.status_bar?.status_message !== undefined ? '✅' : '❌');
    console.log('  status_bar.version:', dataTree?.status_bar?.version !== undefined ? '✅' : '❌');

} catch (error) {
    console.error('❌ Error testing data structure:', error.message);
}