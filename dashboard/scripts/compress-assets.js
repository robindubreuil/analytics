#!/usr/bin/env node

/**
 * Post-build compression script
 * Creates pre-compressed versions of static assets (.gz, .br)
 * Uses native compression tools for maximum efficiency
 */

import { execSync } from 'child_process';
import { readdir, stat } from 'fs/promises';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const DIST_DIR = join(__dirname, '../dist');

// Compression configuration
const COMPRESSORS = {
  gzip: {
    ext: '.gz',
    cmd: 'gzip',
    args: ['-9', '-k', '-f'], // -9: max compression, -k: keep original, -f: force
    check: () => checkCommand('gzip')
  },
  brotli: {
    ext: '.br',
    cmd: 'brotli',
    args: ['-q', '11', '-k', '-f'], // -q 11: max compression, -k: keep original, -f: force
    check: () => checkCommand('brotli')
  }
};

// File extensions to compress
const COMPRESS_EXTENSIONS = new Set([
  '.html',
  '.css',
  '.js',
  '.json',
  '.svg',
  '.xml',
  '.webmanifest',
  '.mjs'
]);

// File size threshold (bytes) - skip very small files
const MIN_SIZE = 200;

async function checkCommand(cmd) {
  try {
    execSync(`command -v ${cmd}`, { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

async function getFilesToCompress(dir) {
  const files = [];
  const entries = await readdir(dir, { withFileTypes: true });

  for (const entry of entries) {
    const fullPath = join(dir, entry.name);

    if (entry.isDirectory()) {
      // Skip hidden directories and common patterns to ignore
      if (!entry.name.startsWith('.') && entry.name !== 'node_modules') {
        files.push(...await getFilesToCompress(fullPath));
      }
    } else if (entry.isFile()) {
      const ext = entry.name.toLowerCase();
      // Check if file has a compressible extension
      const hasValidExt = [...COMPRESS_EXTENSIONS].some(e => ext.endsWith(e));
      // Skip already compressed files
      const isAlreadyCompressed = ext.endsWith('.gz') || ext.endsWith('.br');

      if (hasValidExt && !isAlreadyCompressed) {
        const stats = await stat(fullPath);
        if (stats.size >= MIN_SIZE) {
          files.push(fullPath);
        }
      }
    }
  }

  return files;
}

function compressFile(filePath, compressor) {
  try {
    const start = Date.now();
    execSync(`${compressor.cmd} ${compressor.args.join(' ')} "${filePath}"`, {
      stdio: 'ignore',
      cwd: dirname(filePath)
    });
    return { success: true, duration: Date.now() - start };
  } catch (error) {
    return { success: false, error: error.message };
  }
}

async function compressAll() {
  console.log(' Starting asset compression...\n');

  // Check available compressors
  const available = {};
  for (const [name, config] of Object.entries(COMPRESSORS)) {
    if (await config.check()) {
      available[name] = config;
      console.log(`   ${name} available`);
    } else {
      console.log(`   ${name} not found (install: apt install ${name})`);
    }
  }

  if (Object.keys(available).length === 0) {
    console.error('\n No compression tools available!');
    process.exit(1);
  }

  console.log('\n Scanning dist/ directory...');
  const files = await getFilesToCompress(DIST_DIR);
  console.log(`   Found ${files.length} files to compress\n`);

  let totalCompressed = 0;
  const startTime = Date.now();

  // Process files in batches (parallelization)
  const BATCH_SIZE = 4;

  for (let i = 0; i < files.length; i += BATCH_SIZE) {
    const batch = files.slice(i, i + BATCH_SIZE);

    await Promise.all(
      batch.map(async (filePath) => {
        const relativePath = filePath.replace(DIST_DIR + '/', '');

        for (const [name, compressor] of Object.entries(available)) {
          const result = compressFile(filePath, compressor);
          if (result.success) {
            totalCompressed++;
          }
        }
      })
    );

    // Progress indicator
    process.stdout.write(`\r   Progress: ${Math.min(i + BATCH_SIZE, files.length)}/${files.length} files`);
  }

  const duration = ((Date.now() - startTime) / 1000).toFixed(2);

  console.log(`\n\n Compression complete!`);
  console.log(`   Total compressed: ${totalCompressed} files`);
  console.log(`   Time: ${duration}s`);
  console.log(`   Formats: ${Object.keys(available).join(', ')}`);

  // Show size comparison - rescanning to get actual current files
  console.log('\n Size comparison (sample):');
  const currentFiles = await getFilesToCompress(DIST_DIR);
  const sampleFiles = currentFiles.slice(0, 3);

  for (const file of sampleFiles) {
    try {
      const relativePath = file.replace(DIST_DIR + '/', '');
      const originalStat = await stat(file);
      console.log(`\n   ${relativePath}`);
      console.log(`      Original: ${formatBytes(originalStat.size)}`);

      for (const [name, compressor] of Object.entries(available)) {
        try {
          const compressedPath = file + compressor.ext;
          const compressedStat = await stat(compressedPath);
          const ratio = ((1 - compressedStat.size / originalStat.size) * 100).toFixed(1);
          console.log(`      ${name.padEnd(8)}: ${formatBytes(compressedStat.size)} (-${ratio}%)`);
        } catch {
          // Compressed file might not exist
        }
      }
    } catch {
      // Original file might no longer exist
    }
  }
}

function formatBytes(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB';
}

// Run
compressAll().catch(console.error);
