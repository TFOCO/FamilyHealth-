import * as SecureStore from 'expo-secure-store';
import i18next from 'i18next';

const TOKEN_KEY = 'auth_token';

/**
 * Saves the authentication token securely.
 */
export async function saveToken(token: string): Promise<void> {
  try {
    await SecureStore.setItemAsync(TOKEN_KEY, token);
  } catch (error) {
    console.error('Error saving secure token:', error);
  }
}

/**
 * Retrieves the stored authentication token.
 */
export async function getToken(): Promise<string | null> {
  try {
    return await SecureStore.getItemAsync(TOKEN_KEY);
  } catch (error) {
    console.error('Error retrieving secure token:', error);
    return null;
  }
}

/**
 * Deletes the stored authentication token.
 */
export async function deleteToken(): Promise<void> {
  try {
    await SecureStore.deleteItemAsync(TOKEN_KEY);
  } catch (error) {
    console.error('Error deleting secure token:', error);
  }
}

/**
 * Custom fetch wrapper that automatically appends:
 * 1. Authorization header (if token exists)
 * 2. Accept-Language header matching the selected app language (e.g. hi, pt, en)
 */
export async function fetchWithAuth(url: string, options: RequestInit = {}): Promise<Response> {
  const token = await getToken();
  const activeLanguage = i18next.language || 'en';

  const headers = new Headers(options.headers || {});
  
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  
  // Set the dynamic language preference header
  headers.set('Accept-Language', activeLanguage);
  
  // Also standard JSON content headers
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  return fetch(url, {
    ...options,
    headers,
  });
}
