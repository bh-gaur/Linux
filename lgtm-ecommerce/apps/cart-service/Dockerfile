# ==========================================
# STAGE 1: Builder
# ==========================================
FROM node:20-alpine AS builder

WORKDIR /app

# Copy manifests
COPY cart-service/package*.json ./cart-service/

# Install dependencies
WORKDIR /app/cart-service
RUN npm install --legacy-peer-deps

# Copy application source code
COPY common /app/common
COPY cart-service/ /app/cart-service/

# Prune devDependencies to keep only production packages
RUN npm prune --omit=dev --legacy-peer-deps

# ==========================================
# STAGE 2: Final Production Runner
# ==========================================
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NODE_PATH=/app/cart-service/node_modules

# Copy only production source code and pruned node_modules from builder
COPY --from=builder /app/common ./common
COPY --from=builder /app/cart-service ./cart-service

# Re-create symlink to allow common/ modules to resolve packages from cart-service/node_modules
RUN ln -s /app/cart-service/node_modules /app/node_modules

WORKDIR /app/cart-service

EXPOSE 8093

# Launch the app directly using node
CMD ["node", "app.js"]
