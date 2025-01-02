# pg-rolegen

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
npm install pg-rolegen
# or
yarn add pg-rolegen
```

## Quick Start

```typescript
import { DatabaseManager } from 'pg-rolegen';

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
