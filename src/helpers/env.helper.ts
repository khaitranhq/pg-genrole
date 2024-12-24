export const POSTGRES_DBS_ENV = 'POSTGRES_DBS';
export const POSTGRES_HOST_ENV = 'POSTGRES_HOST';
export const POSTGRES_PORT_ENV = 'POSTGRES_PORT';
export const POSTGRES_USER_ENV = 'POSTGRES_USER';
export const POSTGRES_PASSWORD_ENV = 'POSTGRES_PASSWORD';

export function getEnvironmentVariable(envName: string): string {
  const value = process.env[envName];
  if (!value) throw new Error(`Environment variable ${envName} not set`);
  return value;
}
