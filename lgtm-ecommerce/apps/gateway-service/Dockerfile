# ==========================================
# STAGE 1: Builder
# ==========================================
FROM node:20-alpine AS builder

WORKDIR /app

# Copy manifests
COPY gateway-service/package*.json ./gateway-service/

# Install dependencies
WORKDIR /app/gateway-service
RUN npm install --legacy-peer-deps

# Copy application source code
COPY common /app/common
COPY gateway-service/ /app/gateway-service/

# Prune devDependencies to keep only production packages
RUN npm prune --omit=dev --legacy-peer-deps

# ==========================================
# STAGE 2: Final Production Runner
# ==========================================
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NODE_PATH=/app/gateway-service/node_modules

# Copy only production source code and pruned node_modules from builder
COPY --from=builder /app/common ./common
COPY --from=builder /app/gateway-service ./gateway-service

# Re-create symlink to allow common/ modules to resolve packages from gateway-service/node_modules
RUN ln -s /app/gateway-service/node_modules /app/node_modules

WORKDIR /app/gateway-service

EXPOSE 8090

# Launch the app directly using node
CMD ["node", "app.js"]
