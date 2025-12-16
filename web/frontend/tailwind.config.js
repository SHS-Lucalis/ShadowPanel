export default {
    content: [
        "./index.html",
        "./js/**/*.{js,ts,jsx,tsx,vue}",
        "./packages/**/*.{js,vue,css}",
    ],
    theme: {
        extend: {},
    },
    plugins: [
        require('@tailwindcss/aspect-ratio'),
    ],
    darkMode: 'selector',
}