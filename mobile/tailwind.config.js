/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./App.{js,jsx,ts,tsx}", "./src/**/*.{js,jsx,ts,tsx}"],
  theme: {
    extend: {
      colors: {
        primary: "#0f766e", // teal-700 for high-quality health theme
        accent: "#f59e0b", // amber-500
      }
    },
  },
  plugins: [],
}
