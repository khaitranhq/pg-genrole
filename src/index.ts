import { Client } from 'pg';

export interface IDatabaseConfig {
  readonly host: string;
  readonly port: number;
  readonly user: string;
  readonly password: string;
  readonly appliedDatabases?: string[];
}

export class DatabaseManager {
  private _databaseConfig: IDatabaseConfig;
  private _globalClient: Client;
  private _isDebugging: boolean;

  constructor(databaseConfig: IDatabaseConfig, isDebugging: boolean = false) {
    this._databaseConfig = databaseConfig;
    this._globalClient = this.getDatabaseClient('postgres');
    this._isDebugging = isDebugging;
  }

  async refreshRolesPermissions(): Promise<void> {
    this.log('Start refreshing roles permissionsss');
    await this._globalClient.connect();

    this.log('Start refreshing roles permissions');
    const existingDatabases: string[] = await this.listDatabases();

    const appliedDatabases: string[] =
      this._databaseConfig.appliedDatabases &&
      this._databaseConfig.appliedDatabases.length > 0
        ? this._databaseConfig.appliedDatabases
        : existingDatabases;

    for (const database of appliedDatabases) {
      if (!existingDatabases.includes(database))
        throw new Error(`Database ${database} not found`);

      await this.grantReadPermission(database, `${database}.read`);
      await this.grantReadWritePermisison(database, `${database}.readwrite`);
    }

    await this._globalClient.end();
  }

  private log(message: string): void {
    if (this._isDebugging) this.log(message);
  }

  private getDatabaseClient(databaseName: string): Client {
    return new Client({
      host: this._databaseConfig.host,
      port: this._databaseConfig.port,
      user: this._databaseConfig.user,
      password: this._databaseConfig.password,
      database: databaseName
    });
  }

  private async listDatabases(): Promise<string[]> {
    const result = await this._globalClient.query(
      "SELECT datname FROM pg_database WHERE datname NOT IN ('postgres', 'template0', 'template1', 'rdsadmin');"
    );
    return result.rows.map((row) => row.datname);
  }

  private async checkRoleExisted(roleName: string): Promise<boolean> {
    const query = `SELECT 1 FROM pg_roles WHERE rolname = $1`;
    const res = await this._globalClient.query(query, [roleName]);
    return res.rows.length > 0;
  }

  private async grantReadWritePermisison(
    database: string,
    role: string
  ): Promise<void> {
    this.log(`Grant permission for database ${database} to role ${role}`);

    const client = await this.getDatabaseClient(database);
    await client.connect();

    if (!(await this.checkRoleExisted(role)))
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
      this.log(`Grant permission for schema ${schema}`);
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
        await client.query(
          `ALTER MATERIALIZED VIEW "${schema}"."${mView}" OWNER TO "${role}";`
        );
      }

      // grant permission for view
      const getViewsResult = await client.query(
        `SELECT viewname FROM pg_catalog.pg_views WHERE schemaname = '${schema}'`
      );
      const views = getViewsResult.rows.map((row) => row.viewname);
      for (const view of views) {
        await client.query(
          `GRANT SELECT ON "${schema}"."${view}" TO "${role}";`
        );
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
    this.log('==================================================');
  }

  private async grantReadPermission(
    database: string,
    role: string
  ): Promise<void> {
    this.log(`Grant read permission for database ${database} to role ${role}`);

    const client = await this.getDatabaseClient(database);
    await client.connect();

    if (!(await this.checkRoleExisted(role)))
      await client.query(`CREATE ROLE "${role}"`);

    const result = await client.query(
      "SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT LIKE '%pg%' AND schema_name != 'information_schema';"
    );
    const schemas = result.rows.map((row) => row.schema_name);

    await client.query(`GRANT CONNECT ON DATABASE "${database}" TO "${role}";`);

    // // grant permission for foreign server
    // const foreignServers = await client.query(
    //   'SELECT srvname FROM pg_foreign_server'
    // );
    // for (const foreignServer of foreignServers.rows) {
    //   await client.query(
    //     `GRANT USAGE ON FOREIGN SERVER "${foreignServer.srvname}" TO "${role}";`
    //   );
    // }

    for (const schema of schemas) {
      this.log(`Grant permission for schema ${schema}`);
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
        await client.query(
          `GRANT SELECT ON "${schema}"."${view}" TO "${role}";`
        );
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
    this.log('==================================================');
  }
}
