import { resolve } from 'path'
import { fileURLToPath } from 'url'
import { dirname } from 'path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const frontendRoot = resolve(__dirname, '../..')

export default {
    content: [
        // Main frontend files
        resolve(frontendRoot, 'index.html'),
        resolve(frontendRoot, 'js/**/*.{js,ts,jsx,tsx,vue}'),
        resolve(frontendRoot, 'packages/**/*.{js,vue,css}'),
        // Debug harness files
        resolve(__dirname, 'index.html'),
        resolve(__dirname, 'src/**/*.{js,ts,vue}'),
    ],
    theme: {
        extend: {},
    },
    plugins: [
        require('@tailwindcss/aspect-ratio'),
    ],
    darkMode: 'selector',
}
