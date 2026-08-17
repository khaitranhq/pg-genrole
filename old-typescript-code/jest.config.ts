import { type Config } from 'jest';

const config: Config = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  testMatch: ['**/*.test.ts', '**/test.ts'],
  testTimeout: 30000,
  moduleFileExtensions: ['js', 'ts']
};

export default config;
