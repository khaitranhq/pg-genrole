FROM node:20-slim AS builder

WORKDIR /app

# Copy package files
COPY package.json pnpm-lock.yaml .npmrc ./

# Install dependencies
RUN npm install -g pnpm && \
    pnpm install --frozen-lockfile --ignore-scripts

# Copy source code
COPY src src
COPY index.ts ./
COPY tsconfig.json ./

# Build the application
RUN pnpm run build

FROM node:20-slim AS prod-depdendencies

WORKDIR /app

# Copy package files
COPY package.json pnpm-lock.yaml .npmrc ./

# Install production dependencies
RUN npm install -g pnpm && \
    pnpm install --prod --frozen-lockfile --ignore-scripts

# Create production image
FROM node:20-alpine

# Set environment variables
ENV NODE_ENV=production

WORKDIR /app

# Copy built files and dependencies
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/package.json ./
COPY --from=prod-depdendencies /app/node_modules ./node_modules

# Create a non-root user and set ownership
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

CMD ["node", "dist/index.js"]
