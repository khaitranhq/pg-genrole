# pg-genrole Testing Strategy

## Overview

This document outlines our comprehensive testing approach for **pg-genrole**, a PostgreSQL role automation tool that creates and manages Read-Only (RO), Read-Write (RW), and Admin roles with proper permissions across different PostgreSQL versions (>=13).

The strategy ensures reliability, security, and compatibility across various PostgreSQL versions using local containerized environments.

## Testing Objectives

### Primary Goals

Our testing strategy focuses on six core objectives:

1. **Permission Verification**
   - Validate exact permissions for each role type (RO, RW, Admin)
   - Ensure compliance with the documented permission matrix
   - Test both granted and denied permissions

2. **Cross-Version Compatibility**
   - Support PostgreSQL versions 13, 14, 15, and 16
   - Test version-specific features and behaviors
   - Validate consistent functionality across versions

3. **Database Object Coverage**
   - Test permissions for all supported PostgreSQL objects
   - Include tables, views, sequences, functions, procedures, and more
   - Validate system-level permissions and constraints

4. **Error Handling & Edge Cases**
   - Test failure scenarios
   - Validate error messages
   - Ensure graceful handling of invalid inputs

5. **Idempotency & Reliability**
   - Support multiple tool executions without breaking existing setups
   - Test role modification and updates
   - Validate cleanup and rollback capabilities

### Success Criteria

- ✅ **100% Permission Matrix Compliance**: All role permissions match documented specifications
- ✅ **Multi-Version Support**: Consistent functionality across PostgreSQL 13-16
- ✅ **Zero Security Vulnerabilities**: No privilege escalation or unauthorized access paths
- ✅ **Performance Standards**: Efficient operation on databases with 1000+ objects
- ✅ **Operational Excellence**: Reliable cleanup, rollback, and error recovery

## Architecture

### Test Infrastructure

Our testing infrastructure provides isolated, reproducible environments across multiple deployment scenarios.

#### Technology Stack

| Component                 | Purpose                       | Implementation                   |
| ------------------------- | ----------------------------- | -------------------------------- |
| **Testcontainers for Go** | Isolated PostgreSQL instances | Local development and CI         |
| **Docker**                | Container orchestration       | Database environment management  |
| **pgx/v5**                | PostgreSQL driver             | Database connections and queries |
| **testify**               | Test assertions               | Structured test validation       |
| **Go testing**            | Parallel execution            | Concurrent test performance      |

#### Environment Types

**Local Container Testing**

- Fast, isolated, version-controlled testing
- Ideal for development and rapid feedback
- Full PostgreSQL feature access
- Minimal infrastructure overhead

```go
var postgresVersions = []string{
    "postgres:13-alpine",
    "postgres:14-alpine",
    "postgres:15-alpine",
    "postgres:16-alpine",
}
```

### Environment Isolation Strategy

#### Container-Based Isolation

- **Fresh Containers**: Each test gets a clean PostgreSQL instance
- **Unique Databases**: Dynamic database names with test identifiers
- **Automatic Cleanup**: Container lifecycle management
- **Port Management**: Dynamic port allocation for parallel testing

#### Schema-Based Isolation

- **Separate Schemas**: Multi-role test isolation within databases
- **Dynamic Creation**: Unique schema names for concurrent tests
- **Clean Boundaries**: Isolated test execution environments

## Test Implementation

### Test Categories

#### 1. Unit Tests (`internal/role/`)

#### 2. End-to-End Tests (`tests/e2e/`)

**Purpose**: Complete user workflow validation and component interaction testing

##### 2.1 Role Creation Tests (`role_creation_test.go`)

- Verify role creation for each type (RO, RW, Admin)
- Validate role existence in PostgreSQL system catalogs

##### 2.2 Permission Verification Tests (`permission_verification_test.go`)

- Systematic testing of each permission matrix entry
- Positive permission validation (granted access)
- Negative permission validation (denied access)

##### 2.3 Cross-Version Compatibility Tests (`version_compatibility_test.go`)

- Parallel execution across all supported PostgreSQL versions
- Version-specific feature validation
- Regression detection for version-dependent behaviors

##### 2.4 CLI Interface Tests (`cli_test.go`)

- Command-line argument parsing and validation
- Configuration file handling and precedence
- Output formatting and structured logging
- Error message clarity and actionability

##### 2.5 Workflow Tests (`workflow_test.go`)

- Complete user scenarios from initialization to completion
- Multi-database operations and batch processing
- Interactive vs. non-interactive execution modes
- Configuration management and environment setup

##### 2.6 Performance Tests (`performance_test.go`)

- Large database handling (1000+ tables/objects)
- Concurrent role creation and management
- Memory usage profiling and optimization
- Operation timing benchmarks and regression detection

### Test Reports & Metrics

#### Coverage Requirements

- **Minimum Code Coverage**: 80% across all packages
- **Critical Path Coverage**: 95% for security-related functions
- **Integration Coverage**: All permission matrix combinations

#### Reporting Standards

- **Coverage Reports**: HTML and XML formats for CI integration
- **Test Reports**: JUnit XML for build pipeline integration
- **Performance Metrics**: Benchmark tracking and trend analysis
- **Security Scanning**: Automated vulnerability detection in CI

#### Quality Gates

- All tests must pass before merge
- No decrease in code coverage
- Performance benchmarks within acceptable thresholds
- Security scan approval required

## Security Testing

### 1. Privilege Escalation Prevention

**Objective**: Ensure strict role boundary enforcement

**Test Coverage**:

- **RO Role Boundaries**: Verify read-only roles cannot gain write permissions
- **RW Role Limits**: Ensure read-write roles cannot gain administrative privileges
- **Admin Role Isolation**: Test admin role privilege scope and limitations
- **Role Inheritance**: Validate inheritance chains don't create security gaps
- **System-Level Isolation**: Ensure roles cannot access unauthorized system functions

**Implementation**:

```go
func TestPrivilegeEscalation(t *testing.T) {
    scenarios := []EscalationTest{
        {
            Name: "RO_to_RW_Escalation",
            Role: "readonly_role",
            AttemptedAction: "INSERT INTO test_table VALUES (1)",
            ShouldFail: true,
        },
        {
            Name: "RW_to_Admin_Escalation",
            Role: "readwrite_role",
            AttemptedAction: "CREATE ROLE new_admin",
            ShouldFail: true,
        },
        // Additional escalation scenarios...
    }
}
```

### 2. SQL Injection Prevention

**Objective**: Validate secure SQL generation and execution

**Test Coverage**:

- **Parameterized Queries**: All dynamic SQL uses proper parameterization
- **Input Sanitization**: User inputs are properly validated and escaped
- **Dynamic SQL Security**: Generated SQL is free from injection vulnerabilities
- **Error Message Security**: Error responses don't leak sensitive information

**Security Validation Process**:

1. **Static Analysis**: Automated code scanning for SQL injection patterns
2. **Dynamic Testing**: Runtime injection attempt validation
3. **Input Fuzzing**: Automated testing with malicious input patterns
4. **Code Review**: Manual security review of all SQL generation code

**Example Security Tests**:

```go
func TestSQLInjectionPrevention(t *testing.T) {
    maliciousInputs := []string{
        "'; DROP TABLE users; --",
        "' OR '1'='1",
        "UNION SELECT password FROM users",
        // Additional injection patterns...
    }

    for _, input := range maliciousInputs {
        t.Run(fmt.Sprintf("Injection_Test_%s", input), func(t *testing.T) {
            // Test that malicious input is safely handled
        })
    }
}
```

### 3. Security Audit & Compliance

**Regular Security Reviews**:

- **Dependency Scanning**: Automated vulnerability detection in dependencies
- **Compliance Validation**: Ensure adherence to security best practices

This comprehensive testing strategy ensures that pg-genrole maintains the highest standards of reliability, security, and performance across all supported PostgreSQL environments.
