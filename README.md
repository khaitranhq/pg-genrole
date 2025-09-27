# Automation tool to create RO, RW and admin roles for PostgreSQL databases

**PostgreSQL Version Compatibility**: >=13

## PostgreSQL Permission Matrix

This matrix shows the permissions granted to different user types (RO, RW, Admin) for various PostgreSQL objects.

### Database Objects Permission Matrix

| PostgreSQL Object         | Permission Type                | Read-Only (RO) | Read-Write (RW) | Admin |
| ------------------------- | ------------------------------ | -------------- | --------------- | ----- |
| **Database**              | CONNECT                        | ✅             | ✅              | ✅    |
|                           | TEMPORARY (create temp tables) | ❌             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
| **Schema**                | USAGE                          | ✅             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
| **Tables**                | SELECT                         | ✅             | ✅              | ✅    |
|                           | INSERT                         | ❌             | ✅              | ✅    |
|                           | UPDATE                         | ❌             | ✅              | ✅    |
|                           | DELETE                         | ❌             | ✅              | ✅    |
|                           | TRUNCATE                       | ❌             | ✅              | ✅    |
|                           | REFERENCES                     | ❌             | ❌              | ✅    |
|                           | TRIGGER                        | ❌             | ❌              | ✅    |
|                           | CREATE (ownership)             | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Views**                 | SELECT                         | ✅             | ✅              | ✅    |
|                           | TRIGGER                        | ❌             | ❌              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Materialized Views**    | SELECT                         | ✅             | ✅              | ✅    |
|                           | REFRESH                        | ❌             | ❌              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Sequences**             | SELECT (currval)               | ✅             | ✅              | ✅    |
|                           | USAGE (nextval, setval)        | ❌             | ✅              | ✅    |
|                           | UPDATE (modify sequence)       | ❌             | ❌              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Functions/Procedures**  | EXECUTE                        | ✅             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Indexes**               | Usage (automatic)              | ✅             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
|                           | REINDEX                        | ❌             | ❌              | ✅    |
| **Types/Domains**         | USAGE                          | ✅             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Foreign Data Wrappers** | USAGE                          | ❌             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Foreign Servers**       | USAGE                          | ❌             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Foreign Tables**        | SELECT                         | ✅             | ✅              | ✅    |
|                           | INSERT                         | ❌             | ✅              | ✅    |
|                           | UPDATE                         | ❌             | ✅              | ✅    |
|                           | DELETE                         | ❌             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Large Objects**         | SELECT (read)                  | ✅             | ✅              | ✅    |
|                           | UPDATE (write)                 | ❌             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | DELETE                         | ❌             | ❌              | ✅    |
| **Tablespaces**           | CREATE                         | ❌             | ❌              | ✅    |
|                           | ALTER                          | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |
| **Extensions**            | USAGE                          | ✅             | ✅              | ✅    |
|                           | CREATE                         | ❌             | ❌              | ✅    |
|                           | DROP                           | ❌             | ❌              | ✅    |

### System-Level Permissions

| Permission Category     | Read-Only (RO) | Read-Write (RW) | Admin |
| ----------------------- | -------------- | --------------- | ----- |
| **Role Management**     |                |                 |       |
| CREATE ROLE             | ❌             | ❌              | ✅    |
| ALTER ROLE              | ❌             | ❌              | ✅    |
| DROP ROLE               | ❌             | ❌              | ✅    |
| GRANT/REVOKE            | ❌             | ❌              | ✅    |
| **Database Management** |                |                 |       |
| CREATE DATABASE         | ❌             | ❌              | ✅    |
| ALTER DATABASE          | ❌             | ❌              | ✅    |
| DROP DATABASE           | ❌             | ❌              | ✅    |
| **Configuration**       |                |                 |       |
| ALTER SYSTEM            | ❌             | ❌              | ✅    |
| SET (session)           | ✅             | ✅              | ✅    |
| **Monitoring**          |                |                 |       |
| View system catalogs    | ✅             | ✅              | ✅    |
| View statistics         | ✅             | ✅              | ✅    |
| **Maintenance**         |                |                 |       |
| VACUUM                  | ❌             | ❌              | ✅    |
| ANALYZE                 | ❌             | ❌              | ✅    |
| REINDEX                 | ❌             | ❌              | ✅    |
| **Backup/Restore**      |                |                 |       |
| pg_dump (logical)       | ✅             | ✅              | ✅    |
| pg_restore              | ❌             | ❌              | ✅    |
| Base backup             | ❌             | ❌              | ✅    |

### Connection Management

| Permission             | Read-Only (RO) | Read-Write (RW) | Admin     |
| ---------------------- | -------------- | --------------- | --------- |
| Connection limit       | Default        | Default         | Unlimited |
| pg_cancel_backend()    | ❌             | ❌              | ✅        |
| pg_terminate_backend() | ❌             | ❌              | ✅        |

### Legend

- ✅ **Granted**: Permission is explicitly granted to this role type
- ❌ **Not Granted**: Permission is not granted to this role type
- **RO (Read-Only)**: Can only read data, suitable for reporting and analytics
- **RW (Read-Write)**: Can read and modify data, suitable for application users
- **Admin**: Full administrative privileges, suitable for database administrators

### Notes

1. **Default Privileges**: The matrix shows explicit permissions. Some PostgreSQL objects have default PUBLIC permissions (e.g., EXECUTE on functions).

2. **Schema-Level Control**: Permissions are typically granted at the schema level and inherit to contained objects through default privileges.

3. **Ownership vs Privileges**: Admin roles may receive ownership of objects they create, granting implicit permissions beyond what's listed.

4. **Security Best Practices**:
   - Always follow the principle of least privilege
   - Regularly audit role permissions
   - Use schema-level permissions for easier management
   - Consider row-level security (RLS) for fine-grained access control

5. **Implementation**: This automation tool creates roles with permissions according to this matrix, ensuring consistent security across PostgreSQL databases.
