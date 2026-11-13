'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { writeDistribution } = require('./export-public');

function exportUsb(root, outputParent) {
  // Windows rejects mkdir on an existing drive root, including recursive calls.
  if (!fs.existsSync(outputParent)) fs.mkdirSync(outputParent, { recursive: true });
  // Each export is fresh, so another test's files are never overwritten.
  const output = fs.mkdtempSync(path.join(outputParent, 'usb-source-'));
  const source = path.join(output, 'mgu-repo', 'noahg-apps');
  writeDistribution(root, source);
  return { output, source };
}

if (require.main === module) {
  try {
    const root = path.resolve(__dirname, '..');
    const result = exportUsb(root, path.join(root, 'dist'));
    console.log(`Copy the mgu-repo folder from ${result.output} to your USB stick's root.`);
    console.log('In the Store, select the noahg-apps USB source and install the desired app and its companion service.');
  } catch (error) {
    console.error(`USB export failed: ${error.message}`);
    process.exitCode = 1;
  }
}

module.exports = { exportUsb };
