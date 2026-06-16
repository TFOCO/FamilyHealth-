export interface EncryptedPayload {
  ciphertext: string; // Base64
  iv: string;         // Base64
  salt: string;       // Base64
}

/**
 * Returns the Crypto interface from the environment.
 * Fallback is provided for older Node.js environments where globalThis.crypto is not defined.
 */
const getCrypto = (): Crypto => {
  if (typeof globalThis !== 'undefined' && globalThis.crypto) {
    return globalThis.crypto as Crypto;
  }
  if (typeof window !== 'undefined' && window.crypto) {
    return window.crypto;
  }
  // Safe require fallback for older Node.js environment during tests
  try {
    const nodeCrypto = require('crypto');
    if (nodeCrypto && nodeCrypto.webcrypto) {
      return nodeCrypto.webcrypto as unknown as Crypto;
    }
  } catch (e) {
    // Ignore error
  }
  throw new Error('Web Crypto API is not supported in this environment');
};

/**
 * Helper to encode an ArrayBuffer to a Base64 string.
 * Uses native btoa if available, with a pure JS fallback.
 */
export function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  const len = bytes.byteLength;
  let binary = '';
  for (let i = 0; i < len; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  if (typeof btoa === 'function') {
    return btoa(binary);
  }
  // Pure JS fallback
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  let result = '';
  let i = 0;
  while (i < len) {
    const byte1 = bytes[i++];
    const byte2 = i < len ? bytes[i++] : NaN;
    const byte3 = i < len ? bytes[i++] : NaN;

    const enc1 = byte1 >> 2;
    const enc2 = ((byte1 & 3) << 4) | (isNaN(byte2) ? 0 : byte2 >> 4);
    const enc3 = isNaN(byte2) ? 64 : ((byte2 & 15) << 2) | (isNaN(byte3) ? 0 : byte3 >> 6);
    const enc4 = isNaN(byte3) ? 64 : byte3 & 63;

    result += chars.charAt(enc1) + chars.charAt(enc2) + 
              (enc3 === 64 ? '=' : chars.charAt(enc3)) + 
              (enc4 === 64 ? '=' : chars.charAt(enc4));
  }
  return result;
}

/**
 * Helper to decode a Base64 string to an ArrayBuffer.
 * Uses native atob if available, with a pure JS fallback.
 */
export function base64ToArrayBuffer(base64: string): ArrayBuffer {
  if (typeof atob === 'function') {
    const binaryString = atob(base64);
    const len = binaryString.length;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }
    return bytes.buffer;
  }
  // Pure JS fallback
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  const lookup = new Uint8Array(256);
  for (let i = 0; i < chars.length; i++) {
    lookup[chars.charCodeAt(i)] = i;
  }
  
  let bufferLength = base64.length * 0.75;
  if (base64.endsWith('==')) {
    bufferLength -= 2;
  } else if (base64.endsWith('=')) {
    bufferLength -= 1;
  }
  
  const bytes = new Uint8Array(bufferLength);
  let p = 0;
  for (let i = 0; i < base64.length; i += 4) {
    const encoded1 = lookup[base64.charCodeAt(i)];
    const encoded2 = lookup[base64.charCodeAt(i + 1)];
    const encoded3 = lookup[base64.charCodeAt(i + 2)];
    const encoded4 = lookup[base64.charCodeAt(i + 3)];

    bytes[p++] = (encoded1 << 2) | (encoded2 >> 4);
    if (p < bufferLength) {
      bytes[p++] = ((encoded2 & 15) << 4) | (encoded3 >> 2);
    }
    if (p < bufferLength) {
      bytes[p++] = ((encoded3 & 3) << 6) | (encoded4 & 63);
    }
  }
  return bytes.buffer;
}

/**
 * Derives a 256-bit AES key from a passphrase and a salt using PBKDF2-HMAC-SHA256.
 */
export async function deriveKey(passphrase: string, salt: Uint8Array): Promise<CryptoKey> {
  const cryptoInstance = getCrypto();
  const passphraseBytes = new TextEncoder().encode(passphrase);

  // Import the raw passphrase as a key material
  const keyMaterial = await cryptoInstance.subtle.importKey(
    'raw',
    passphraseBytes,
    'PBKDF2',
    false,
    ['deriveKey']
  );

  // Derive the AES key using PBKDF2
  return await cryptoInstance.subtle.deriveKey(
    {
      name: 'PBKDF2',
      salt: salt,
      iterations: 100000,
      hash: 'SHA-256',
    },
    keyMaterial,
    {
      name: 'AES-GCM',
      length: 256,
    },
    false,
    ['encrypt', 'decrypt']
  );
}

/**
 * Encrypts a string plaintext with a key derived from the user passphrase.
 * Returns the Base64-encoded ciphertext, salt, and IV.
 */
export async function encryptPayload(passphrase: string, plaintext: string): Promise<EncryptedPayload> {
  const cryptoInstance = getCrypto();
  
  // Generate a random 16-byte salt and 12-byte initialization vector (IV)
  const salt = cryptoInstance.getRandomValues(new Uint8Array(16));
  const iv = cryptoInstance.getRandomValues(new Uint8Array(12));

  // Derive the encryption key
  const aesKey = await deriveKey(passphrase, salt);

  // Encrypt the plaintext
  const plaintextBytes = new TextEncoder().encode(plaintext);
  const ciphertextBuffer = await cryptoInstance.subtle.encrypt(
    {
      name: 'AES-GCM',
      iv: iv,
    },
    aesKey,
    plaintextBytes
  );

  return {
    ciphertext: arrayBufferToBase64(ciphertextBuffer),
    iv: arrayBufferToBase64(iv.buffer),
    salt: arrayBufferToBase64(salt.buffer),
  };
}

/**
 * Decrypts a Base64-encoded encrypted payload using the key derived from the user passphrase.
 * Returns the original string plaintext.
 */
export async function decryptPayload(passphrase: string, encrypted: EncryptedPayload): Promise<string> {
  const cryptoInstance = getCrypto();

  const saltBytes = new Uint8Array(base64ToArrayBuffer(encrypted.salt));
  const ivBytes = new Uint8Array(base64ToArrayBuffer(encrypted.iv));
  const ciphertextBuffer = base64ToArrayBuffer(encrypted.ciphertext);

  // Derive the decryption key
  const aesKey = await deriveKey(passphrase, saltBytes);

  // Decrypt the ciphertext
  const decryptedBuffer = await cryptoInstance.subtle.decrypt(
    {
      name: 'AES-GCM',
      iv: ivBytes,
    },
    aesKey,
    ciphertextBuffer
  );

  return new TextDecoder().decode(decryptedBuffer);
}

/**
 * Helper to encrypt a JavaScript object.
 */
export async function encryptObject<T>(passphrase: string, payload: T): Promise<EncryptedPayload> {
  const jsonStr = JSON.stringify(payload);
  return encryptPayload(passphrase, jsonStr);
}

/**
 * Helper to decrypt a JavaScript object.
 */
export async function decryptObject<T>(passphrase: string, encrypted: EncryptedPayload): Promise<T> {
  const jsonStr = await decryptPayload(passphrase, encrypted);
  return JSON.parse(jsonStr) as T;
}
