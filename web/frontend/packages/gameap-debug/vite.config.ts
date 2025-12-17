import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { viteCommonjs } from '@originjs/vite-plugin-commonjs'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))

// Path to the main frontend
const frontendRoot = resolve(__dirname, '../..')

// Custom plugin to resolve @gameap/plugin-sdk from anywhere
function pluginSdkResolver(sdkPath: string): Plugin {
    return {
        name: 'plugin-sdk-resolver',
        enforce: 'pre',
        resolveId(source) {
            if (source === '@gameap/plugin-sdk') {
                return sdkPath
            }
            return null
        },
    }
}

// Default plugin path - can be overridden via PLUGIN_PATH env variable
function resolvePluginPath(): string {
    const pluginPath = process.env.PLUGIN_PATH

    if (!pluginPath) {
        // Default to hex-editor-plugin's built bundle
        return resolve(__dirname, '../../../../../hex-editor-plugin/frontend/dist')
    }

    if (pluginPath.startsWith('/')) {
        // Absolute path
        return pluginPath
    }

    // Relative path from current working directory
    return resolve(process.cwd(), pluginPath)
}

// Use the built plugin-sdk
const pluginSdkPath = resolve(__dirname, '../../../plugin-sdk/dist/index.js')

export default defineConfig({
    plugins: [
        pluginSdkResolver(pluginSdkPath),
        viteCommonjs(),
        vue(),
    ],
    root: __dirname,
    base: '/',
    publicDir: resolve(frontendRoot, 'public'),
    resolve: {
        alias: [
            // Debug harness source (for mocks, etc.)
            { find: '@debug', replacement: resolve(__dirname, 'src') },
            // Real frontend JS directory (for app.js and other real components)
            { find: '@app', replacement: resolve(frontendRoot, 'js') },
            // Standard @ alias for real frontend
            { find: '@', replacement: resolve(frontendRoot, 'js') },
            // GameAP UI package
            { find: '@gameap/ui', replacement: resolve(frontendRoot, 'packages/gameap-ui') },
            // Plugin SDK - use regex to match from anywhere
            { find: /^@gameap\/plugin-sdk$/, replacement: pluginSdkPath },
            // Plugin source (built bundle)
            { find: '@plugin', replacement: resolvePluginPath() },
        ],
    },
    css: {
        // Use debug harness's postcss config with correct Tailwind content paths
        postcss: resolve(__dirname, 'postcss.config.cjs'),
        preprocessorOptions: {
            scss: {
                api: 'modern-compiler',
            },
        },
    },
    server: {
        port: 5174,
        open: true,
        fs: {
            // Allow serving files from these directories
            allow: [
                __dirname,
                frontendRoot,
                resolve(__dirname, '../../../plugin-sdk'),
                resolvePluginPath(),
                resolve(frontendRoot, 'node_modules'),
            ],
        },
    },
    define: {
        'window.gameapLang': JSON.stringify(process.env.LOCALE || 'en'),
    },
    optimizeDeps: {
        include: [
            'vue',
            'vue-router',
            'pinia',
            'axios',
            'naive-ui',
            'dayjs',
            'codemirror',
        ],
        // Don't pre-bundle the plugin SDK to allow alias resolution
        exclude: ['@gameap/plugin-sdk', 'msw'],
        esbuildOptions: {
            // Ensure esbuild also resolves the SDK correctly
            alias: {
                '@gameap/plugin-sdk': pluginSdkPath,
            },
        },
    },
    build: {
        outDir: 'dist',
        emptyOutDir: true,
    },
})
