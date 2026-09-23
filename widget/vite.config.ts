import { defineConfig } from 'vite'
import { resolve } from 'path'

export default defineConfig({
  build: {
    // IIFE ไฟล์เดียว ไม่มี dependency ตอน runtime
    // เพราะไปฝังในหน้าเว็บคนอื่น จะไปพึ่ง module loader ของเขาไม่ได้
    lib: {
      entry: resolve(__dirname, 'src/index.ts'),
      name: 'AIOffice',
      formats: ['iife'],
      fileName: () => 'ai-office.v1.js',
    },
    outDir: resolve(__dirname, '../static/widget'),
    emptyOutDir: false,
    target: 'es2019',
  },
  test: {
    environment: 'happy-dom',
    globals: true,
  },
})
