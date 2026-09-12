<!-- admin-portal/src/App.vue -->
<template>
  <div class="admin-container">
    <h1>OGame Next-Gen: Administration Portal</h1>
    
    <div class="monitor-grid">
      <!-- Extended Cluster Status Card -->
      <div class="card">
        <h3>System Infrastructure Status</h3>
        <div class="services-list">
          <div class="service-item-extended" v-for="(metrics, name) in systemStatuses" :key="name">
            <div class="service-main-row">
              <span class="service-name">{{ formatServiceName(name) }}</span>
              <span :class="['status-badge', metrics.status ? 'online' : 'offline']">
                {{ metrics.status ? 'ONLINE' : 'OFFLINE' }}
              </span>
            </div>
            <!-- Sub-metrics visible only if the service is online -->
            <div class="service-metrics-row" v-if="metrics.status">
              <span class="metric-tag">Version: <strong>{{ metrics.version }}</strong></span>
              <span class="metric-tag">Uptime: <strong>{{ metrics.uptime }}</strong></span>
            </div>
          </div>
        </div>
      </div>

      <!-- Action Panel Card -->
      <div class="card">
        <h3>Database Migrations</h3>
        <p>Control center for running direct raw SQL schemas and remote migration scripts.</p>
        <button class="btn-disabled" disabled>Execute Pending Migrations</button>
        <p class="footnote">Status: Pending Phase 4 architecture connection.</p>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, ref, onMounted, onBeforeUnmount } from 'vue';

export default defineComponent({
  name: 'App',
  setup() {
    // Extended tracking state for each infrastructure layer
    const systemStatuses = ref({
      admin_portal: { status: true, version: 'v0.1.0', uptime: 'Live' },
      gateway: { status: false, version: 'N/A', uptime: 'N/A' },
      game_core: { status: false, version: 'N/A', uptime: 'N/A' },
      postgres: { status: false, version: 'N/A', uptime: 'N/A' },
      redis: { status: false, version: 'N/A', uptime: 'N/A' }
    });

    let pollingInterval: number | null = null;
    const startTime = Date.now();

    const formatServiceName = (name: string) => {
      return name.replace('_', ' ').toUpperCase();
    };

    const calculateAdminUptime = () => {
      const diff = Date.now() - startTime;
      const secs = Math.floor(diff / 1000) % 60;
      const mins = Math.floor(diff / (1000 * 60)) % 60;
      const hours = Math.floor(diff / (1000 * 60 * 60)) % 24;
      systemStatuses.value.admin_portal.uptime = `${hours}h ${mins}m ${secs}s`;
    };

    const checkClusterHealth = async () => {
      calculateAdminUptime();
      try {
        const response = await fetch('http://localhost:8080/api/admin/health');
        if (!response.ok) throw new Error('Gateway unhealthy');
        
        const data = await response.json();
        
        // Update extended structural telemetry from Go response mappings
        systemStatuses.value.gateway = data.gateway;
        systemStatuses.value.game_core = data.game_core;
        systemStatuses.value.postgres = data.postgres;
        systemStatuses.value.redis = data.redis;
      } catch (error) {
        // Safe cluster reset to OFFLINE if the proxy fails
        const offlineState = { status: false, version: 'N/A', uptime: 'N/A' };
        systemStatuses.value.gateway = offlineState;
        systemStatuses.value.game_core = offlineState;
        systemStatuses.value.postgres = offlineState;
        systemStatuses.value.redis = offlineState;
      }
    };

    onMounted(() => {
      checkClusterHealth();
      pollingInterval = window.setInterval(checkClusterHealth, 2000); // Polling every 2 seconds for fresh live uptime numbers
    });

    onBeforeUnmount(() => {
      if (pollingInterval) clearInterval(pollingInterval);
    });

    return {
      systemStatuses,
      formatServiceName
    };
  }
});
</script>

<style scoped>
.admin-container {
  padding: 30px;
  font-family: 'Segoe UI', Roboto, sans-serif;
  background: #0f172a;
  color: #f8fafc;
  min-height: 100vh;
}
h1 {
  color: #38bdf8;
  border-bottom: 2px solid #1e293b;
  padding-bottom: 15px;
  margin-bottom: 30px;
}
.monitor-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
  gap: 25px;
}
.card {
  background: #1e293b;
  padding: 24px;
  border-radius: 8px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.4);
  border: 1px solid #334155;
}
h3 {
  margin-top: 0;
  color: #94a3b8;
  border-bottom: 1px solid #334155;
  padding-bottom: 10px;
}
.services-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 15px;
}
.service-item-extended {
  background: #0f172a;
  border-radius: 6px;
  padding: 12px;
  border-left: 4px solid #475569;
}
.service-main-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.service-name {
  font-weight: 600;
  font-size: 14px;
  letter-spacing: 0.05em;
}
.service-metrics-row {
  display: flex;
  gap: 20px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid #1e293b;
  font-size: 12px;
  color: #94a3b8;
}
.status-badge {
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: bold;
}
.status-badge.online {
  background: #064e3b;
  color: #34d399;
  border: 1px solid #059669;
}
.status-badge.offline {
  background: #7f1d1d;
  color: #fca5a5;
  border: 1px solid #dc2626;
}
.btn-disabled {
  background: #475569;
  color: #94a3b8;
  padding: 10px 20px;
  border: none;
  border-radius: 4px;
  cursor: not-allowed;
  font-weight: bold;
}
.footnote {
  font-size: 12px;
  color: #64748b;
  margin-top: 10px;
}
</style>
