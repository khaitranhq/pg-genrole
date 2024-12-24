import { Client } from 'pg';

const POSTGRES_DBS_ENV = 'POSTGRES_DBS';
const POSTGRES_HOST_ENV = 'POSTGRES_HOST';
const POSTGRES_PORT_ENV = 'POSTGRES_PORT';
const POSTGRES_USER_ENV = 'POSTGRES_USER';
const POSTGRES_PASSWORD_ENV = 'POSTGRES_PASSWORD';

function getEnvironmentVariable(envName: string): string {
  const value = process.env[envName];
  if (!value) throw new Error(`Environment variable ${envName} not set`);
  return value;
}

function getClientDatabase(database: string): Client {
  const host = getEnvironmentVariable(POSTGRES_HOST_ENV);
  const port = parseInt(getEnvironmentVariable(POSTGRES_PORT_ENV));
  const user = getEnvironmentVariable(POSTGRES_USER_ENV);
  const password = getEnvironmentVariable(POSTGRES_PASSWORD_ENV);

  return new Client({
    host,
    port,
    user,
    password,
    database
  });
}

async function listDatabases(client: Client): Promise<string[]> {
  const result = await client.query(
    "SELECT datname FROM pg_database WHERE datname NOT IN ('postgres', 'template0', 'template1', 'rdsadmin');"
  );
  return result.rows.map((row) => row.datname);
}

async function checkRoleExisted(
  client: Client,
  roleName: string
): Promise<boolean> {
  const query = `SELECT 1 FROM pg_roles WHERE rolname = $1`;
  const res = await client.query(query, [roleName]);

  return res.rows.length > 0;
}

async function grantReadWritePermisison(
  database: string,
  role: string
): Promise<void> {
  console.log(`Grant permission for database ${database} to role ${role}`);

  const client = await getClientDatabase(database);
  await client.connect();

  if (!(await checkRoleExisted(client, role)))
    await client.query(`CREATE ROLE "${role}"`);

  const result = await client.query(
    "SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT LIKE '%pg%' AND schema_name != 'information_schema';"
  );
  const schemas = result.rows.map((row) => row.schema_name);

  await client.query(`GRANT CONNECT ON DATABASE "${database}" TO "${role}";`);

  // grant permission for foreign server
  const foreignServers = await client.query(
    'SELECT srvname FROM pg_foreign_server'
  );
  for (const foreignServer of foreignServers.rows) {
    await client.query(
      `GRANT USAGE ON FOREIGN SERVER "${foreignServer.srvname}" TO "${role}";`
    );
  }

  for (const schema of schemas) {
    console.log(`Grant permission for schema ${schema}`);
    await client.query(`GRANT USAGE ON SCHEMA "${schema}" TO "${role}"`);

    // change permission for tables
    await client.query(
      `GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON ALL TABLES IN SCHEMA "${schema}" TO "${role}";`
    );
    await client.query(
      `ALTER DEFAULT PRIVILEGES IN SCHEMA "${schema}"
				GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON TABLES TO "${role}";`
    );

    // grant permission for matviews
    const getMViewsResult = await client.query(
      `SELECT matviewname FROM pg_matviews WHERE schemaname = '${schema}'`
    );
    const mViews = getMViewsResult.rows.map((row) => row.matviewname);
    for (const mView of mViews) {
      await client.query(
        `GRANT SELECT ON "${schema}"."${mView}" TO "${role}";`
      );
    }

    // grant permission for view
    const getViewsResult = await client.query(
      `SELECT viewname FROM pg_catalog.pg_views WHERE schemaname = '${schema}'`
    );
    const views = getViewsResult.rows.map((row) => row.viewname);
    for (const view of views) {
      await client.query(`GRANT SELECT ON "${schema}"."${view}" TO "${role}";`);
    }

    // grant permissions for sequences
    await client.query(
      `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA "${schema}" TO "${role}";`
    );

    await client.query(
      `ALTER DEFAULT PRIVILEGES IN SCHEMA "${schema}"
				GRANT USAGE, SELECT ON SEQUENCES TO "${role}";`
    );

    // grant permission for functions
    await client.query(
      `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA "${schema}" TO "${role}"`
    );
    await client.query(
      `ALTER DEFAULT PRIVILEGES IN SCHEMA "${schema}"
				GRANT EXECUTE ON FUNCTIONS TO "${role}";`
    );
  }

  await client.end();
  console.log('==================================================');
}

async function grantReadPermission(
  database: string,
  role: string
): Promise<void> {
  console.log(`Grant read permission for database ${database} to role ${role}`);

  const client = await getClientDatabase(database);
  await client.connect();

  if (!(await checkRoleExisted(client, role)))
    await client.query(`CREATE ROLE "${role}"`);

  const result = await client.query(
    "SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT LIKE '%pg%' AND schema_name != 'information_schema';"
  );
  const schemas = result.rows.map((row) => row.schema_name);

  await client.query(`GRANT CONNECT ON DATABASE "${database}" TO "${role}";`);

  // grant permission for foreign server
  const foreignServers = await client.query(
    'SELECT srvname FROM pg_foreign_server'
  );
  for (const foreignServer of foreignServers.rows) {
    await client.query(
      `GRANT USAGE ON FOREIGN SERVER "${foreignServer.srvname}" TO "${role}";`
    );
  }

  for (const schema of schemas) {
    console.log(`Grant permission for schema ${schema}`);
    await client.query(`GRANT USAGE ON SCHEMA "${schema}" TO "${role}"`);

    // change permission for tables
    await client.query(
      `GRANT SELECT ON ALL TABLES IN SCHEMA "${schema}" TO "${role}";`
    );
    await client.query(
      `ALTER DEFAULT PRIVILEGES IN SCHEMA "${schema}"
				GRANT SELECT ON TABLES TO "${role}";`
    );

    // grant permission for matviews
    const getMViewsResult = await client.query(
      `SELECT matviewname FROM pg_matviews WHERE schemaname = '${schema}'`
    );
    const mViews = getMViewsResult.rows.map((row) => row.matviewname);
    for (const mView of mViews) {
      await client.query(
        `GRANT SELECT ON "${schema}"."${mView}" TO "${role}";`
      );
    }

    // grant permission for view
    const getViewsResult = await client.query(
      `SELECT viewname FROM pg_catalog.pg_views WHERE schemaname = '${schema}'`
    );
    const views = getViewsResult.rows.map((row) => row.viewname);
    for (const view of views) {
      await client.query(`GRANT SELECT ON "${schema}"."${view}" TO "${role}";`);
    }

    // grant permission for sequences
    await client.query(
      `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA "${schema}" TO "${role}";`
    );

    await client.query(
      `ALTER DEFAULT PRIVILEGES IN SCHEMA "${schema}"
				GRANT USAGE, SELECT ON SEQUENCES TO "${role}";`
    );

    // grant permission for functions
    await client.query(
      `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA "${schema}" TO "${role}"`
    );
    await client.query(
      `ALTER DEFAULT PRIVILEGES IN SCHEMA "${schema}"
				GRANT EXECUTE ON FUNCTIONS TO "${role}";`
    );
  }

  await client.end();
  console.log('==================================================');
}

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
