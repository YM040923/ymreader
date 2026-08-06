/**
 * Uint8Array ES2024 Hex/Base64 方法的类型声明
 * pdfjs-dist 5.x 使用了这些方法，低版本浏览器需要 polyfill
 * 此声明文件让 TypeScript 编译器识别这些 polyfill 方法
 */

interface Uint8Array {
  /** 将 Uint8Array 转换为十六进制字符串 */
  toHex(): string;
  /** 从十六进制字符串设置 Uint8Array 内容 */
  setFromHex(hexString: string): { read: number; written: number };
  /** 将 Uint8Array 转换为 Base64 字符串 */
  toBase64(): string;
  /** 从 Base64 字符串设置 Uint8Array 内容 */
  setFromBase64(base64String: string): { read: number; written: number };
}

interface Uint8ArrayConstructor {
  /** 从十六进制字符串创建 Uint8Array */
  fromHex(hexString: string): Uint8Array;
  /** 从 Base64 字符串创建 Uint8Array */
  fromBase64(base64String: string): Uint8Array;
}
