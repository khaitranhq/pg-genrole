import { DatabaseManager, IDatabaseConfig } from './src';

function getEnv(key: string): string {
  const value = process.env[key];
  if (!value) {
    throw new Error(`Environment variable ${key} is not set`);
  }
  return value;
}

async function main() {
  const listDatabasesEnvValue = process.env.LIST_DATABASES;
  const listDatabases = listDatabasesEnvValue
    ? listDatabasesEnvValue.split(',')
    : undefined;

  const isDebugging = process.env.DEBUG === 'true';

  const config: IDatabaseConfig = {
    host: getEnv('DB_HOST'),
    port: parseInt(getEnv('DB_PORT')),
    user: getEnv('DB_USER'),
    password: getEnv('DB_PASSWORD')
  };
  const databaseManager = new DatabaseManager(config, isDebugging);
  await databaseManager.refreshRolesPermissions(listDatabases);
}

main();
