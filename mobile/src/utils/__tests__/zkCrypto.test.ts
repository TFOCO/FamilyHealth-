import { 
  encryptPayload, 
  decryptPayload, 
  encryptObject, 
  decryptObject, 
  deriveKey,
  arrayBufferToBase64,
  base64ToArrayBuffer
} from '../zkCrypto';

describe('Zero-Knowledge Cryptography Utility (zkCrypto)', () => {
  const passphrase = 'SuperSecretSecurePassword123!';
  const phiPayload = {
    patientName: 'Jane Doe',
    dateOfBirth: '1988-04-12',
    diagnosis: 'Type 2 Diabetes Mellitus',
    prescriptions: ['Metformin 500mg daily'],
    vitals: {
      systolic: 120,
      diastolic: 80,
      heartRate: 72
    }
  };

  test('Base64 Conversion Helpers encode and decode correctly', () => {
    const originalText = 'Hello World of Zero Knowledge!';
    const encoder = new TextEncoder();
    const decoder = new TextDecoder();

    const buffer = encoder.encode(originalText).buffer;
    const base64 = arrayBufferToBase64(buffer);
    const decodedBuffer = base64ToArrayBuffer(base64);
    const decodedText = decoder.decode(decodedBuffer);

    expect(decodedText).toBe(originalText);
    expect(base64).toBe(btoa ? btoa(originalText) : 'SGVsbG8gV29ybGQgb2YgWmVybyBLbm93bGVkZ2Uh');
  });

  test('deriveKey produces a CryptoKey instance', async () => {
    const salt = new Uint8Array(16);
    // Fill with sample salt bytes
    for (let i = 0; i < 16; i++) salt[i] = i;

    const key = await deriveKey(passphrase, salt);
    expect(key).toBeDefined();
    expect(key.type).toBe('secret');
    expect(key.algorithm.name).toBe('AES-GCM');
  });

  test('encryptPayload and decryptPayload encrypt and restore string plaintext', async () => {
    const plaintext = 'Sensitive Patient PHI: BP 140/90, Glucose 126 mg/dL';
    const encrypted = await encryptPayload(passphrase, plaintext);

    expect(encrypted.ciphertext).toBeDefined();
    expect(encrypted.iv).toBeDefined();
    expect(encrypted.salt).toBeDefined();

    // Ciphertext should not contain the original plaintext
    expect(encrypted.ciphertext).not.toContain('Sensitive');

    const decrypted = await decryptPayload(passphrase, encrypted);
    expect(decrypted).toBe(plaintext);
  });

  test('encryptObject and decryptObject encrypt and restore JavaScript object payloads', async () => {
    const encrypted = await encryptObject(passphrase, phiPayload);

    expect(encrypted.ciphertext).toBeDefined();
    expect(encrypted.iv).toBeDefined();
    expect(encrypted.salt).toBeDefined();

    const decrypted = await decryptObject<typeof phiPayload>(passphrase, encrypted);
    expect(decrypted).toEqual(phiPayload);
    expect(decrypted.patientName).toBe('Jane Doe');
    expect(decrypted.vitals.systolic).toBe(120);
  });

  test('Decrypting with an incorrect passphrase throws an error', async () => {
    const plaintext = 'Top Secret Vitals';
    const encrypted = await encryptPayload(passphrase, plaintext);

    // Try decrypting with wrong passphrase
    const wrongPassphrase = 'WrongPassword!';
    await expect(decryptPayload(wrongPassphrase, encrypted)).rejects.toThrow();
  });

  test('Modifying ciphertext/tampering triggers a decryption error', async () => {
    const plaintext = 'Tamper Proof Data';
    const encrypted = await encryptPayload(passphrase, plaintext);

    // Corrupt the ciphertext slightly by decoding, changing a byte, and encoding back
    const bytes = new Uint8Array(base64ToArrayBuffer(encrypted.ciphertext));
    bytes[0] ^= 0xFF; // flip bits of the first byte
    const tamperedCiphertext = arrayBufferToBase64(bytes.buffer);

    const tamperedPayload = {
      ...encrypted,
      ciphertext: tamperedCiphertext
    };

    await expect(decryptPayload(passphrase, tamperedPayload)).rejects.toThrow();
  });
});
