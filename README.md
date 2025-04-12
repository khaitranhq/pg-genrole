# pg-genrole

A Node.js package that automates the creation and management of PostgreSQL roles and permissions across multiple databases.

## Features

- 🚀 Automatically creates read and readwrite roles for each database
- 🔒 Implements secure permission management following PostgreSQL best practices
- 🎯 Supports multiple schemas (app, audit, etc.)
- 📊 Handles tables, views, sequences, functions, and materialized views
- 🔄 Manages default privileges for new objects
- ⚡ Easy to integrate with existing PostgreSQL setups

## Installation

```bash
npm install pg-genrole
# or
yarn add pg-genrole
# or
pnpm add pg-genrole
```

## Quick Start

### Using as a Node.js Package

```typescript
import { DatabaseManager } from 'pg-genrole';

const config = {
  host: 'localhost',
  port: 5432,
  user: 'postgres',
  password: 'your_password'
};

const manager = new DatabaseManager(config);

// Create roles for specific databases
await manager.refreshRolesPermissions(['db1', 'db2']);
```

### Using Docker Image

You can also use the pre-built Docker image to manage your PostgreSQL roles:

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
    
  genrole:
    image: khaitranhq/pg-genrole:latest
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      DB_USER: postgres
      DB_PASSWORD: postgres
      # Optional: Specify databases to manage (comma-separated)
      # LIST_DATABASES: db1,db2
      # Optional: Enable debug logging
      DEBUG: true
    depends_on:
      - postgres
```

Run with:

```bash
docker-compose up -d
```

The Docker image will automatically:
1. Connect to your PostgreSQL server
2. Create appropriate read and readwrite roles for each database
3. Set up all necessary permissions

## Permissions Overview

### Read Role (`{database}.read`)

- Connect to database
- Read-only access to specified schemas
- SELECT on:
  - Tables
  - Views
  - Materialized Views
  - Sequences

### ReadWrite Role (`{database}.readwrite`)

- All read role permissions
- Write access to schemas
- INSERT, UPDATE, DELETE on tables
- USAGE on sequences
- EXECUTE on functions
- REFRESH on materialized views
- TRIGGER permissions

## Configuration Options

```typescript
interface DatabaseManagerConfig {
  host: string;
  port: number;
  user: string;
  password: string;
}
```

## Best Practices

1. Always use separate roles for read and write access
2. Grant minimum necessary permissions
3. Use connection pooling in production
4. Regularly audit role permissions
5. Keep role passwords secure
6. Backup databases before running this package

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
