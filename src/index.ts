import { Client } from 'pg';

import {
  getClientDatabase,
  grantReadPermission,
  grantReadWritePermisison,
  listDatabases
} from './helpers/database.helper';
import { POSTGRES_DBS_ENV } from './helpers/env.helper';

export async function refreshRolesPermissions() {
  console.log('Start refreshing roles permissions');

  const client: Client = await getClientDatabase('postgres');
  await client.connect();

  const refreshDatabaseEnvValue: string | undefined =
    process.env[POSTGRES_DBS_ENV];

  const existingDatabases: string[] = await listDatabases(client);
  let refreshDatabases: string[] = existingDatabases;

  if (refreshDatabaseEnvValue)
    refreshDatabases = refreshDatabaseEnvValue.split(',');

  for (const database of refreshDatabases) {
    if (!existingDatabases.includes(database))
      throw new Error(`Database ${database} not found`);

    await grantReadPermission(database, `${database}.read`);
    await grantReadWritePermisison(database, `${database}.readwrite`);
  }

  await client.end();
}
