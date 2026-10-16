#!/usr/bin/env node
// Merges jest coverage-final.json files (e.g. one per CI shard) and writes the HTML report, a json-summary
// (read by coverage_summary.py) and an lcov file. It uses the istanbul libraries jest already installs in
// webapp/node_modules, so it needs no extra dependency.
//
//   node scripts/antimatter/merge_jest_coverage.js <out-dir> <coverage-final.json>...

'use strict';

const fs = require('fs');
const path = require('path');
const {createRequire} = require('module');

const root = path.resolve(__dirname, '..', '..');
const webappRequire = createRequire(path.join(root, 'webapp', 'package.json'));
const libCoverage = webappRequire('istanbul-lib-coverage');
const libReport = webappRequire('istanbul-lib-report');
const reports = webappRequire('istanbul-reports');

const [outDir, ...inputs] = process.argv.slice(2);
if (!outDir || inputs.length === 0) {
    console.error('usage: merge_jest_coverage.js <out-dir> <coverage-final.json>...');
    process.exit(2);
}

const map = libCoverage.createCoverageMap({});
let merged = 0;
for (const input of inputs) {
    if (!fs.existsSync(input)) {
        console.warn(`skipping missing ${input}`);
        continue;
    }
    map.merge(JSON.parse(fs.readFileSync(input, 'utf8')));
    merged++;
}
if (merged === 0) {
    console.error('no coverage file to merge');
    process.exit(1);
}

fs.mkdirSync(outDir, {recursive: true});
fs.writeFileSync(path.join(outDir, 'coverage-final.json'), JSON.stringify(map.toJSON()));

const context = libReport.createContext({dir: outDir, coverageMap: map, defaultSummarizer: 'nested'});
for (const name of ['html', 'json-summary', 'lcovonly', 'text-summary']) {
    reports.create(name, name === 'html' ? {subdir: 'html'} : {}).execute(context);
}
console.log(`merged ${merged} coverage file(s) into ${outDir}`);
