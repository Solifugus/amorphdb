#!/usr/bin/env node

// Simple verification script to check basic PWA functionality fixes
// This doesn't test DOM manipulation, but tests the core logic fixes

// Mock DOM environment for testing
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
global.localStorage = {
    getItem: () => null,
    setItem: () => {}
};
global.EventSource = function() { this.close = () => {}; };
global.queueMicrotask = (fn) => setTimeout(fn, 0);
global.fetch = () => Promise.resolve({ ok: true });

// Load the AmorphPWA class (simulate loading from file)
const fs = require('fs');
const path = require('path');

// Read the PWA file and extract just the class definition
const pwaFile = fs.readFileSync(path.join(__dirname, 'amorphdb-pwa.js'), 'utf8');

// Define AmorphPWA class in this context
let AmorphPWA;
eval(pwaFile);

console.log('🧪 Running PWA fixes verification...\n');

// Test the fixes
async function runTests() {
    let passed = 0;
    let failed = 0;

    function assert(condition, message) {
        if (condition) {
            console.log(`✅ ${message}`);
            passed++;
        } else {
            console.log(`❌ ${message}`);
            failed++;
        }
    }

    try {
        // Test 1: Section tree parsing with null check
        console.log('🔍 Test 1: Section Tree Parsing Null Check');
        const pwa = new AmorphPWA();

        // This should not crash
        const result = pwa.generateDataNode(null);
        assert(typeof result === 'object' && result !== null, 'generateDataNode handles null input gracefully');

        // Test with undefined
        const result2 = pwa.generateDataNode(undefined);
        assert(typeof result2 === 'object' && result2 !== null, 'generateDataNode handles undefined input gracefully');

        // Test 2: Path handling in processTemplate
        console.log('\n🔍 Test 2: Path Array Handling');

        // Mock a section tree
        const mockSectionTree = {
            name: 'test',
            layout: 'vertical',
            type: null,
            template: '<span>{field}</span>',
            fields: ['field'],
            children: {}
        };

        pwa.sectionTree = mockSectionTree;
        const dataTree = pwa.generateDataTree();
        assert(dataTree && typeof dataTree === 'object', 'Data tree generation works');

        // Test processTemplate with array path
        const processed1 = pwa.processTemplate('<span>{field}</span>', ['test'], { field: 'value' });
        assert(typeof processed1 === 'string', 'processTemplate works with array path');

        // Test processTemplate with string path (should not crash)
        const processed2 = pwa.processTemplate('<span>{field}</span>', 'test', { field: 'value' });
        assert(typeof processed2 === 'string', 'processTemplate works with string path');

        console.log('\n🔍 Test 3: Local Dev Mode');

        // Test forceLocalDevMode
        pwa.forceLocalDevMode();
        assert(pwa.localDevMode === true, 'forceLocalDevMode sets localDevMode to true');

        console.log('\n🔍 Test 4: Proxy Tracking');

        // Test proxy creation and change tracking
        pwa.rawData = { test: 'value' };
        pwa.proxiedData = pwa.createProxy(pwa.rawData);

        const initialChanges = pwa.changes.size;
        pwa.proxiedData.test = 'new value';
        assert(pwa.changes.size > initialChanges, 'Proxy tracks changes');
        assert(pwa.changes.has('test'), 'Proxy tracks the correct field');

        console.log('\n🔍 Test 5: Server Update Application');

        // Test applyServerUpdate
        const oldChangeSize = pwa.changes.size;
        pwa.applyServerUpdate('test', 'server value');
        assert(pwa.changes.size === oldChangeSize, 'Server updates do not trigger change tracking');
        assert(pwa.rawData.test === 'server value', 'Server updates modify raw data');

        console.log('\n📊 Test Results:');
        console.log(`✅ Passed: ${passed}`);
        console.log(`❌ Failed: ${failed}`);
        console.log(`📈 Success Rate: ${passed}/${passed + failed} (${Math.round(passed / (passed + failed) * 100)}%)`);

        if (failed === 0) {
            console.log('\n🎉 All core fixes verified! The integration test should now pass.');
        } else {
            console.log('\n⚠️  Some issues remain. Check the failed tests above.');
        }

    } catch (error) {
        console.error('❌ Test execution failed:', error.message);
        failed++;
    }
}

runTests().catch(console.error);