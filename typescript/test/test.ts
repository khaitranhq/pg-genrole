import { execSync } from 'child_process';
import { Client } from 'pg';

import { DatabaseManager, IDatabaseConfig } from '../src';

const defaultDatabaseConfig: IDatabaseConfig = {
  host: '127.0.0.1',
  port: 5432,
  user: 'postgres',
  password: 'postgres'
};

const TEST_READ_USERNAME = 'test_read_user';
const TEST_READWRITE_USERNAME = 'test_readwrite_user';
const TEST_PASSWORD = 'test_password';
const TEST_DATABASES = ['db1', 'db2', 'db3'];

describe('DatabaseManager Integration Tests', () => {
  let globalClient: Client;
  let databaseManager: DatabaseManager;

  beforeEach(async () => {
    execSync('docker compose -f ./test/compose.yml up -d', {
      stdio: 'inherit'
    });

    // Wait for PostgreSQL to be ready
    await new Promise((resolve) => setTimeout(resolve, 3000));

    // Connect to postgres database to create user and databases
    globalClient = new Client({
      ...defaultDatabaseConfig,
      database: 'postgres'
    });

    await globalClient.connect();

    // Create test users
    await globalClient.query(
      `CREATE USER ${TEST_READ_USERNAME} WITH PASSWORD '${TEST_PASSWORD}'`
    );
    await globalClient.query(
      `CREATE USER ${TEST_READWRITE_USERNAME} WITH PASSWORD '${TEST_PASSWORD}'`
    );

    // Create test databases
    for (const dbName of TEST_DATABASES) {
      console.log(`\nCreate mock data for database: ${dbName}`);
      await globalClient.query(`DROP DATABASE IF EXISTS ${dbName}`);
      await globalClient.query(`CREATE DATABASE ${dbName}`);

      // Connect to the newly created database
      const dbClient = new Client({
        ...defaultDatabaseConfig,
        database: dbName
      });
      await dbClient.connect();

      // Create schemas
      await dbClient.query(`
            CREATE SCHEMA IF NOT EXISTS app;
            CREATE SCHEMA IF NOT EXISTS audit;
          `);

      // Create sequences
      await dbClient.query(`
            CREATE SEQUENCE app.user_id_seq
              INCREMENT 1
              START 1000
              MINVALUE 1000
              MAXVALUE 9999999999;
    
            CREATE SEQUENCE app.post_id_seq
              INCREMENT 1
              START 1
              MINVALUE 1
              MAXVALUE 9999999999;
          `);

      // Create tables
      await dbClient.query(`
            CREATE TABLE app.users (
              id INTEGER PRIMARY KEY DEFAULT nextval('app.user_id_seq'),
              email VARCHAR(255) UNIQUE NOT NULL,
              username VARCHAR(50) NOT NULL,
              role VARCHAR(20) DEFAULT 'user',
              created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
              updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
            );
    
            CREATE TABLE app.posts (
              id INTEGER PRIMARY KEY DEFAULT nextval('app.post_id_seq'),
              user_id INTEGER REFERENCES app.users(id),
              title VARCHAR(200) NOT NULL,
              content TEXT,
              status VARCHAR(20) DEFAULT 'draft',
              created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
              updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
            );
    
            CREATE TABLE app.comments (
              id SERIAL PRIMARY KEY,
              post_id INTEGER REFERENCES app.posts(id),
              user_id INTEGER REFERENCES app.users(id),
              content TEXT NOT NULL,
              created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
            );
    
            CREATE TABLE audit.changes (
              id SERIAL PRIMARY KEY,
              table_name VARCHAR(50),
              record_id INTEGER,
              action VARCHAR(20),
              changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
              old_data JSONB,
              new_data JSONB
            );
          `);

      // Create functions
      await dbClient.query(`
            CREATE OR REPLACE FUNCTION update_updated_at_column()
            RETURNS TRIGGER AS $$
            BEGIN
              NEW.updated_at = CURRENT_TIMESTAMP;
              RETURN NEW;
            END;
            $$ language 'plpgsql';
    
            CREATE OR REPLACE FUNCTION audit.log_changes()
            RETURNS TRIGGER AS $$
            BEGIN
              IF TG_OP = 'UPDATE' THEN
                INSERT INTO audit.changes (table_name, record_id, action, old_data, new_data)
                VALUES (TG_TABLE_NAME, OLD.id, 'UPDATE', to_jsonb(OLD), to_jsonb(NEW));
              ELSIF TG_OP = 'DELETE' THEN
                INSERT INTO audit.changes (table_name, record_id, action, old_data)
                VALUES (TG_TABLE_NAME, OLD.id, 'DELETE', to_jsonb(OLD));
              END IF;
              RETURN NULL;
            END;
            $$ LANGUAGE plpgsql;
    
            CREATE OR REPLACE FUNCTION app.get_user_stats(p_user_id INTEGER)
            RETURNS TABLE (
              post_count INTEGER,
              comment_count INTEGER,
              last_activity TIMESTAMP WITH TIME ZONE
            ) AS $$
            BEGIN
              RETURN QUERY
              SELECT 
                COUNT(DISTINCT p.id)::INTEGER as post_count,
                COUNT(DISTINCT c.id)::INTEGER as comment_count,
                MAX(GREATEST(COALESCE(p.created_at, '1970-01-01'), COALESCE(c.created_at, '1970-01-01'))) as last_activity
              FROM app.users u
              LEFT JOIN app.posts p ON p.user_id = u.id
              LEFT JOIN app.comments c ON c.user_id = u.id
              WHERE u.id = p_user_id;
            END;
            $$ LANGUAGE plpgsql;
          `);

      // Create triggers
      await dbClient.query(`
            CREATE TRIGGER update_users_modtime
              BEFORE UPDATE ON app.users
              FOR EACH ROW
              EXECUTE FUNCTION update_updated_at_column();
    
            CREATE TRIGGER update_posts_modtime
              BEFORE UPDATE ON app.posts
              FOR EACH ROW
              EXECUTE FUNCTION update_updated_at_column();
    
            CREATE TRIGGER audit_users_changes
              AFTER UPDATE OR DELETE ON app.users
              FOR EACH ROW
              EXECUTE FUNCTION audit.log_changes();
    
            CREATE TRIGGER audit_posts_changes
              AFTER UPDATE OR DELETE ON app.posts
              FOR EACH ROW
              EXECUTE FUNCTION audit.log_changes();
          `);

      // Create views
      await dbClient.query(`
            CREATE VIEW app.active_users AS
            SELECT 
              u.id,
              u.username,
              u.email,
              COUNT(DISTINCT p.id) as post_count,
              COUNT(DISTINCT c.id) as comment_count,
              MAX(GREATEST(p.created_at, c.created_at)) as last_activity
            FROM app.users u
            LEFT JOIN app.posts p ON p.user_id = u.id
            LEFT JOIN app.comments c ON c.user_id = u.id
            GROUP BY u.id, u.username, u.email;
    
            CREATE MATERIALIZED VIEW app.post_stats AS
            SELECT 
              date_trunc('day', created_at) as post_date,
              status,
              COUNT(*) as post_count,
              COUNT(DISTINCT user_id) as unique_authors
            FROM app.posts
            GROUP BY date_trunc('day', created_at), status
            WITH DATA;
          `);

      // Create indexes
      await dbClient.query(`
            CREATE INDEX idx_posts_user_id ON app.posts(user_id);
            CREATE INDEX idx_comments_post_id ON app.comments(post_id);
            CREATE INDEX idx_comments_user_id ON app.comments(user_id);
            CREATE INDEX idx_post_stats_date ON app.post_stats(post_date);
          `);

      // Insert sample data
      await dbClient.query(`
            INSERT INTO app.users (email, username, role) VALUES 
              ('user1@${dbName}.com', 'user1_${dbName}', 'admin'),
              ('user2@${dbName}.com', 'user2_${dbName}', 'user'),
              ('user3@${dbName}.com', 'user3_${dbName}', 'user');
    
            INSERT INTO app.posts (user_id, title, content, status)
            SELECT 
              u.id,
              'Post ' || generate_series || ' by ' || u.username,
              'Content for post ' || generate_series || ' in database ' || '${dbName}',
              CASE WHEN generate_series % 2 = 0 THEN 'published' ELSE 'draft' END
            FROM app.users u
            CROSS JOIN generate_series(1, 3);
    
            INSERT INTO app.comments (post_id, user_id, content)
            SELECT 
              p.id,
              u.id,
              'Comment on post ' || p.id || ' by ' || u.username
            FROM app.posts p
            CROSS JOIN app.users u
            WHERE p.status = 'published';
          `);

      // Refresh materialized views to reflect the sample data
      await dbClient.query(`
            REFRESH MATERIALIZED VIEW app.post_stats;
          `);

      await dbClient.end();
    }
  });

  describe('Generate roles for all databases', () => {
    beforeEach(async () => {
      databaseManager = new DatabaseManager(defaultDatabaseConfig);
      await databaseManager.refreshRolesPermissions();

      for (const database of TEST_DATABASES) {
        await globalClient.query(
          `GRANT "${database}.read" TO ${TEST_READ_USERNAME}`
        );

        await globalClient.query(
          `GRANT "${database}.readwrite" TO ${TEST_READWRITE_USERNAME}`
        );
      }
    });

    it('should generate enough roles for all databases', async () => {
      console.log('\n=== Testing role generation ===');
      const result = await globalClient.query(`SELECT rolname FROM pg_roles`);

      const existingRoles = result.rows.map((row) => row.rolname);
      const expectedReadRoles = TEST_DATABASES.map(
        (dbName) => `${dbName}.read`
      );
      const expectedReadWriteRoles = TEST_DATABASES.map(
        (dbName) => `${dbName}.readwrite`
      );

      for (const role of [...expectedReadRoles, ...expectedReadWriteRoles]) {
        expect(existingRoles).toContain(role);
      }
    });

    it('should grant correct permissions to read roles', async () => {
      console.log('\n=== Testing read permissions ===');
      // Test for each database
      for (const dbName of TEST_DATABASES) {
        // Create client with read user credentials
        const readClient = new Client({
          ...defaultDatabaseConfig,
          database: dbName,
          user: TEST_READ_USERNAME,
          password: TEST_PASSWORD
        });

        await readClient.connect();

        try {
          console.log('1. Testing Sequences...');
          // 1. Test Sequences
          const sequencesResult = await readClient.query(`
            SELECT 
              sequence_schema,
              sequence_name,
              start_value,
              minimum_value,
              maximum_value,
              increment
            FROM information_schema.sequences
            WHERE sequence_schema = 'app'
            ORDER BY sequence_name;
          `);

          expect(sequencesResult.rows).toHaveLength(3);

          // Check user_id_seq
          const userSeq = sequencesResult.rows.find(
            (row) => row.sequence_name === 'user_id_seq'
          );
          expect(userSeq).toBeDefined();
          expect(userSeq).toMatchObject({
            sequence_schema: 'app',
            start_value: '1000',
            minimum_value: '1000',
            maximum_value: '9999999999',
            increment: '1'
          });

          console.log('2. Testing Tables Read Access...');
          // 2. Test Tables Read Access
          // Should be able to read from tables
          const usersResult = await readClient.query(
            'SELECT * FROM app.users LIMIT 1'
          );
          expect(usersResult.rows).toHaveLength(1);
          expect(usersResult.rows[0]).toHaveProperty('email');
          expect(usersResult.rows[0]).toHaveProperty('username');

          // Should NOT be able to modify tables
          await expect(
            readClient.query(`
              INSERT INTO app.users (email, username, role) 
              VALUES ('test@test.com', 'test', 'user')
            `)
          ).rejects.toThrow();

          console.log('3. Testing Functions...');
          // 3. Test Functions
          const userStatsResult = await readClient.query(
            `
            SELECT * FROM app.get_user_stats($1)
          `,
            [usersResult.rows[0].id]
          );
          expect(userStatsResult.rows[0]).toHaveProperty('post_count');
          expect(userStatsResult.rows[0]).toHaveProperty('comment_count');
          expect(userStatsResult.rows[0]).toHaveProperty('last_activity');

          console.log('4. Testing Views...');
          // 4. Test Views
          const activeUsersResult = await readClient.query(
            'SELECT * FROM app.active_users LIMIT 1'
          );
          expect(activeUsersResult.rows).toHaveLength(1);
          expect(activeUsersResult.rows[0]).toHaveProperty('username');
          expect(activeUsersResult.rows[0]).toHaveProperty('post_count');
          expect(activeUsersResult.rows[0]).toHaveProperty('comment_count');

          // Should NOT be able to modify views
          await expect(
            readClient.query(
              'CREATE OR REPLACE VIEW app.active_users AS SELECT 1 as dummy'
            )
          ).rejects.toThrow();

          console.log('5. Testing Materialized Views...');
          // 5. Test Materialized Views
          const postStatsResult = await readClient.query(
            'SELECT * FROM app.post_stats LIMIT 1'
          );
          expect(postStatsResult.rows).toHaveLength(1);
          expect(postStatsResult.rows[0]).toHaveProperty('post_date');
          expect(postStatsResult.rows[0]).toHaveProperty('status');
          expect(postStatsResult.rows[0]).toHaveProperty('post_count');
          expect(postStatsResult.rows[0]).toHaveProperty('unique_authors');

          // Should NOT be able to refresh materialized view
          await expect(
            readClient.query('REFRESH MATERIALIZED VIEW app.post_stats')
          ).rejects.toThrow();

          console.log('6. Testing Sequence Operations...');
          // 6. Verify sequence operations
          const currentVal = await readClient.query(`
            SELECT last_value FROM app.user_id_seq;
          `);
          expect(currentVal.rows[0].last_value).toBeDefined();

          // Should NOT be able to modify sequence
          await expect(
            readClient.query(`
              ALTER SEQUENCE app.user_id_seq RESTART WITH 1;
            `)
          ).rejects.toThrow();
        } catch (error) {
          console.error(`\nError in database ${dbName}:`, error);
        } finally {
          await readClient.end();
        }
      }
    });

    it('should set correct ownership of materialized views', async () => {
      console.log('\n=== Testing materialized view ownership ===');
      for (const dbName of TEST_DATABASES) {
        const dbClient = new Client({
          ...defaultDatabaseConfig,
          database: dbName
        });
        await dbClient.connect();

        const ownershipQuery = `
          SELECT mv.schemaname, mv.matviewname, mv.matviewowner as owner
          FROM pg_matviews mv
          INNER JOIN pg_roles r ON r.rolname = mv.matviewowner
          WHERE mv.schemaname = 'app'
        `;

        const result = await dbClient.query(ownershipQuery);
        await dbClient.end();

        // Check post_stats materialized view ownership
        const postStats = result.rows.find(
          (row) => row.matviewname === 'post_stats'
        );
        expect(postStats).toBeDefined();
        expect(postStats?.owner).toBe(`${dbName}.readwrite`);
      }
    });

    it('should grant correct permissions to readwrite roles', async () => {
      console.log('\n=== Testing readwrite permissions ===');
      // Test for each database
      for (const dbName of TEST_DATABASES) {
        console.log(`\nTesting database: ${dbName}`);

        // Create client with readwrite user credentials
        const readwriteClient = new Client({
          ...defaultDatabaseConfig,
          database: dbName,
          user: TEST_READWRITE_USERNAME,
          password: TEST_PASSWORD
        });
        try {
          await readwriteClient.connect();
          console.log('1. Testing Table Operations...');
          // Insert new user
          const insertResult = await readwriteClient.query(`
            INSERT INTO app.users (email, username, role) 
            VALUES ('test@test.com', 'test_user', 'user')
            RETURNING id, email, username, role;
          `);
          expect(insertResult.rows[0]).toMatchObject({
            email: 'test@test.com',
            username: 'test_user',
            role: 'user'
          });

          // Update user
          await readwriteClient.query(`
            UPDATE app.users 
            SET username = 'updated_user'
            WHERE email = 'test@test.com';
          `);

          // Verify update
          const updateResult = await readwriteClient.query(`
            SELECT username FROM app.users WHERE email = 'test@test.com';
          `);
          expect(updateResult.rows[0].username).toBe('updated_user');

          // Delete user
          await readwriteClient.query(`
            DELETE FROM app.users WHERE email = 'test@test.com';
          `);

          console.log('2. Testing View Operations...');
          // Should be able to read from views
          const viewResult = await readwriteClient.query(`
            SELECT * FROM app.active_users LIMIT 1;
          `);
          expect(viewResult.rows).toHaveLength(1);

          // Should NOT be able to create/modify views
          await expect(
            readwriteClient.query(`
            CREATE OR REPLACE VIEW app.test_view AS SELECT 1 as dummy;
          `)
          ).rejects.toThrow();

          console.log('3. Testing Materialized View Operations...');
          // Should be able to read from materialized views
          const matViewResult = await readwriteClient.query(`
            SELECT * FROM app.post_stats LIMIT 1;
          `);
          expect(matViewResult.rows).toHaveLength(1);

          // Should be able to refresh materialized views
          await readwriteClient.query(`
            REFRESH MATERIALIZED VIEW app.post_stats;
          `);

          console.log('4. Testing Function Execution...');
          // Should be able to execute functions
          const userId = viewResult.rows[0].id;
          const statsResult = await readwriteClient.query(
            `
            SELECT * FROM app.get_user_stats($1);
          `,
            [userId]
          );
          expect(statsResult.rows[0]).toHaveProperty('post_count');

          console.log('5. Testing Sequence Operations...');
          // Should be able to read sequences
          const seqResult = await readwriteClient.query(`
            SELECT last_value FROM app.user_id_seq;
          `);
          expect(seqResult.rows[0].last_value).toBeDefined();

          // Should NOT be able to modify sequences
          await expect(
            readwriteClient.query(`
            ALTER SEQUENCE app.user_id_seq RESTART WITH 1;
          `)
          ).rejects.toThrow();

          console.log('6. Testing Schema Operations...');
          // Should NOT be able to create new schemas
          await expect(
            readwriteClient.query(`
            CREATE SCHEMA test_schema;
          `)
          ).rejects.toThrow();

          // Should NOT be able to modify schema
          await expect(
            readwriteClient.query(`
            ALTER SCHEMA app RENAME TO test_app;
          `)
          ).rejects.toThrow();

          console.log('7. Testing Trigger Behavior...');
          // Insert a user and verify updated_at is set by trigger
          const triggerTest = await readwriteClient.query(`
            INSERT INTO app.users (email, username, role)
            VALUES ('trigger@test.com', 'trigger_test', 'user')
            RETURNING updated_at;
          `);
          expect(triggerTest.rows[0].updated_at).toBeDefined();

          // Update user and verify updated_at changes
          const beforeUpdate = triggerTest.rows[0].updated_at;
          await new Promise((resolve) => setTimeout(resolve, 1000)); // Wait 1 second

          await readwriteClient.query(`
            UPDATE app.users 
            SET username = 'trigger_updated'
            WHERE email = 'trigger@test.com'
            RETURNING updated_at;
          `);

          const afterUpdate = await readwriteClient.query(`
            SELECT updated_at 
            FROM app.users 
            WHERE email = 'trigger@test.com';
          `);

          expect(afterUpdate.rows[0].updated_at).not.toEqual(beforeUpdate);

          // Clean up trigger test user
          await readwriteClient.query(`
            DELETE FROM app.users WHERE email = 'trigger@test.com';
          `);
        } catch (error) {
          console.error(`\nError in database ${dbName}:`, error);
        } finally {
          await readwriteClient.end();
        }
      }
    });
  });

  describe('Generate roles for specific databases', () => {
    beforeEach(async () => {
      databaseManager = new DatabaseManager(defaultDatabaseConfig);
      await databaseManager.refreshRolesPermissions(['db1', 'db2']);

      // Grant roles to test users for db1 and db2 only
      for (const database of ['db1', 'db2']) {
        await globalClient.query(
          `GRANT "${database}.read" TO ${TEST_READ_USERNAME}`
        );
        await globalClient.query(
          `GRANT "${database}.readwrite" TO ${TEST_READWRITE_USERNAME}`
        );
      }
    });

    it('should generate roles only for specified databases', async () => {
      const result = await globalClient.query(`SELECT rolname FROM pg_roles`);
      const existingRoles = result.rows.map((row) => row.rolname);

      // Should have roles for db1 and db2
      expect(existingRoles).toContain('db1.read');
      expect(existingRoles).toContain('db1.readwrite');
      expect(existingRoles).toContain('db2.read');
      expect(existingRoles).toContain('db2.readwrite');

      // Should NOT have roles for db3
      expect(existingRoles).not.toContain('db3.read');
      expect(existingRoles).not.toContain('db3.readwrite');
    });

    it('should grant correct permissions for specified databases only', async () => {
      // Test access to db1 (should work)
      const db1Client = new Client({
        ...defaultDatabaseConfig,
        database: 'db1',
        user: TEST_READ_USERNAME,
        password: TEST_PASSWORD
      });
      await db1Client.connect();
      const db1Result = await db1Client.query(
        'SELECT * FROM app.users LIMIT 1'
      );
      expect(db1Result.rows).toHaveLength(1);
      await db1Client.end();

      // Test access to db2 (should work)
      const db2Client = new Client({
        ...defaultDatabaseConfig,
        database: 'db2',
        user: TEST_READ_USERNAME,
        password: TEST_PASSWORD
      });
      await db2Client.connect();
      const db2Result = await db2Client.query(
        'SELECT * FROM app.users LIMIT 1'
      );
      expect(db2Result.rows).toHaveLength(1);
      await db2Client.end();

      // Test access to db3 (should fail)
      const db3Client = new Client({
        ...defaultDatabaseConfig,
        database: 'db3',
        user: TEST_READ_USERNAME,
        password: TEST_PASSWORD
      });
      try {
        db3Client.connect();
        const result = await db3Client.query('SELECT * FROM app.users LIMIT 1');
        console.log('Result:', result);
      } catch (error) {
        // Make sure we got the expected error
        expect(error).toBeInstanceOf(Error);
        if (error instanceof Error)
          expect(error.message).toContain('permission denied');
      } finally {
        await db3Client.end();
      }
    });

    it('should maintain correct permissions when refreshing roles', async () => {
      // First refresh - already done in beforeEach

      // Verify initial permissions
      const initialResult = await globalClient.query(
        `SELECT rolname FROM pg_roles`
      );
      const initialRoles = initialResult.rows.map((row) => row.rolname);

      // Do another refresh
      databaseManager = new DatabaseManager(defaultDatabaseConfig);
      await databaseManager.refreshRolesPermissions(['db1', 'db2']);

      // Verify permissions remain the same
      const finalResult = await globalClient.query(
        `SELECT rolname FROM pg_roles`
      );
      const finalRoles = finalResult.rows.map((row) => row.rolname);

      // Should have same roles before and after
      expect(finalRoles).toEqual(expect.arrayContaining(initialRoles));

      // Should still not have db3 roles
      expect(finalRoles).not.toContain('db3.read');
      expect(finalRoles).not.toContain('db3.readwrite');
    });
  });

  afterEach(async () => {
    await globalClient.end();
    execSync('docker compose -f ./test/compose.yml down --volumes', {
      stdio: 'inherit'
    });

    // Wait for PostgreSQL to be ready
    // await new Promise((resolve) => setTimeout(resolve, 5000));
  });
});
