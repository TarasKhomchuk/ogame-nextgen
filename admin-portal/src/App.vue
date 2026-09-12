<!-- admin-portal/src/App.vue -->
<template>
  <div class="admin-container">
    <h1>OGame Next-Gen: Administration Portal</h1>
    
    <div class="monitor-grid">
      
      <!-- Card 1: System Infrastructure Status -->
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
            
            <div class="service-metrics-row" v-if="metrics.status">
              <span class="metric-tag">Version: <strong>{{ metrics.version }}</strong></span>
              <span class="metric-tag">Uptime: <strong>{{ metrics.uptime }}</strong></span>
              
              <!-- Clean implementation: Render ONLY if an active target database name exists -->
              <span class="metric-tag" v-if="name === 'postgres' && metrics.database_state">
                Active Database: <strong style="color: #34d399;">{{ metrics.database_state }}</strong>
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Card 2: Database Cluster Control Center -->
      <div class="card">
        <h3>Database Cluster Management</h3>
        <p class="section-desc">Provision storage nodes, perform hot-swaps, and inject server structure blueprints.</p>
        
        <div class="form-group">
          <label for="db-name-input">Target Database Name:</label>
          <div class="input-group">
            <input 
              id="db-name-input"
              v-model="targetDbName" 
              type="text" 
              placeholder="e.g., ogame_game_db, ogame_dev_sandbox" 
              :disabled="loading"
            />
            <button @click="executeDbSwitch" :disabled="loading || !targetDbName" class="btn-primary">
              Mount / Create DB
            </button>
          </div>
        </div>

        <div class="migration-block">
          <h4>Schema Blueprint Operations</h4>
          <p class="footnote">Populate the current active database context with tables schema from disk array.</p>
          <button @click="executeSchemaMigration" :disabled="loading || isDbDisconnected" class="btn-accent">
            Execute SQL Schema Migration
          </button>
        </div>

        <div v-if="operationMessage" :class="['status-banner', operationSuccess ? 'success-banner' : 'error-banner']">
          {{ operationMessage }}
        </div>
      </div>
    </div>

    <!-- Row 2: Live Database Catalog Inventory -->
    <div class="catalog-container" style="margin-top: 30px;">
      <div class="card">
        <h3>PostgreSQL Instance Catalog Inventory</h3>
        <p class="section-desc">Real-time database records registered inside the active PostgreSQL cluster node.</p>

        <div v-if="dbCatalog.length === 0" class="empty-catalog">
          No custom game databases found. Use the panel above to mount your first cluster database.
        </div>

        <table v-else class="catalog-table">
          <thead>
            <tr>
              <th>Database Name</th>
              <th>Status Mapping</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="db in dbCatalog" :key="db.name" :class="{'active-row': db.is_active }">
              <td class="db-name-cell">📁 {{ db.name }}</td>
              <td>
                <span :class="['badge-indicator', db.is_active ? 'badge-active' : 'badge-idle']">
                  {{ db.is_active ? 'ACTIVE ROUTE' : 'IDLE DISCONNECTED' }}
                </span>
              </td>
              <td>
                <button 
                  @click="quickSwitch(db.name)"
                  :disabled="loading || db.is_active"
                  class="btn-small"
                >
                  Hot-Swap Route
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

  <script lang="ts">
    import {defineComponent, ref, computed, onMounted, onBeforeUnmount} from 'vue';

    interface DatabaseDetails {
      name: string;
      is_active: boolean;
    }

    export default defineComponent({
      name: 'App',
      setup() {
        // Structural states object tracking cluster entities
        const systemStatuses = ref({
          admin_portal: {status: true, version: 'v0.1.0-vue', uptime: 'Live' },
        gateway: {status: false, version: 'N/A', uptime: 'N/A' },
        game_core: {status: false, version: 'N/A', uptime: 'N/A' },
        postgres: {status: false, version: 'DISCONNECTED', uptime: 'N/A' },
        redis: {status: false, version: 'N/A', uptime: 'N/A' }
        });

        const dbCatalog = ref<DatabaseDetails[]>([]);
        const targetDbName = ref('');
        const loading = ref(false);
        const operationMessage = ref('');
        const operationSuccess = ref(true);

        let pollingInterval: number | null = null;
        const startTime = Date.now();

        // Computed flag to evaluate block restrictions
        const isDbDisconnected = computed(() => {
          return systemStatuses.value.postgres.version === 'DISCONNECTED';
        });

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

        // Query Go Gateway for the entire cluster databases array list
        const fetchDatabaseCatalog = async () => {
          try {
            const response = await fetch('http://localhost:8080/api/admin/databases');
        if (response.ok) {
              const data = await response.json();
        dbCatalog.value = data || [];
            }
          } catch (e) {
          dbCatalog.value = [];
          }
        };

        const checkClusterHealth = async () => {
          calculateAdminUptime();
          try {
              const response = await fetch('http://localhost:8080/api/admin/health');
          if (!response.ok) throw new Error('Gateway unreachable');

          const data = await response.json();
          systemStatuses.value.gateway = data.gateway;
          systemStatuses.value.game_core = data.game_core;
          systemStatuses.value.postgres = data.postgres;
          systemStatuses.value.redis = data.redis;
            } catch (error) {
              const offlineState = {status: false, version: 'N/A', uptime: 'N/A' };
          systemStatuses.value.gateway = offlineState;
          systemStatuses.value.game_core = offlineState;
          systemStatuses.value.postgres = {status: false, version: 'DISCONNECTED', uptime: 'N/A' };
          systemStatuses.value.redis = offlineState;
            }
        };

        const executeDbSwitch = async () => {
          if (!targetDbName.value) return;
          loading.value = true;
          operationMessage.value = '';

          try {
                const response = await fetch('http://localhost:8080/api/admin/databases/switch', {
                  method: 'POST',
                  headers: {'Content-Type': 'application/json' },
                  body: JSON.stringify({database_name: targetDbName.value })
                });

            const result = await response.json();
            operationSuccess.value = result.success;
            operationMessage.value = result.message;

            if (result.success) {
              targetDbName.value = '';
              await fetchDatabaseCatalog();
              await checkClusterHealth();
            }
          } catch (e) {
            operationSuccess.value = false;
            operationMessage.value = 'Failed to execute cluster database routing command.';
          } finally {
            loading.value = false;
          }
        };

        const quickSwitch = async (name: string) => {
          targetDbName.value = name;
          await executeDbSwitch();
        };

        const executeSchemaMigration = async () => {
          loading.value = true;
          operationMessage.value = '';

          try {
            const response = await fetch('http://localhost:8080/api/admin/databases/migrate', {
              method: 'POST',
              headers: {'Content-Type': 'application/json' },
              body: JSON.stringify({admin_username: 'placeholder', admin_password: 'placeholder' })
            });

            const result = await response.json();
            operationSuccess.value = result.success;
            operationMessage.value = result.message;
          } catch (e) {
            operationSuccess.value = false;
            operationMessage.value = 'Failed to transmit migration pipeline packets.';
          } finally {
            loading.value = false;
          }
        };

        onMounted(() => {
          checkClusterHealth();
          fetchDatabaseCatalog();

          pollingInterval = window.setInterval(() => {
            checkClusterHealth();
            fetchDatabaseCatalog();
          }, 2000);
        });

        onBeforeUnmount(() => {if (pollingInterval) clearInterval(pollingInterval);});
        return {systemStatuses, dbCatalog, targetDbName, loading, operationMessage, operationSuccess, isDbDisconnected, formatServiceName, executeDbSwitch, quickSwitch, executeSchemaMigration};
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
    grid-template-columns: repeat(auto-fit, minmax(380px, 1fr));
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
    color: #f1f5f9;
    border-bottom: 1px solid #334155;
    padding-bottom: 10px;
}

.section-desc {
    font-size: 13px;
    color: #94a3b8;
    margin-top: -5px;
    margin-bottom: 20px;
}

.services-list {
    display: flex;
    flex-direction: column;
    gap: 14px;
}

.service-item-extended {
    background: #0f172a;
    border-radius: 6px;
    padding: 12px;
    border-left: 4px solid #38bdf8;
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

.form-group {
    margin-bottom: 20px;
}

label {
    display: block;
    font-size: 13px;
    color: #94a3b8;
    margin-bottom: 6px;
}

.input-group {
    display: flex;
    gap: 10px;
}

input {
    flex: 1;
    background: #0f172a;
    border: 1px solid #334155;
    padding: 10px;
    border-radius: 4px;
    color: #fff;
    font-size: 14px;
}

input:focus {
    border-color: #38bdf8;
    outline: none;
}

.btn-primary {
    background: #0284c7;
    color: #fff;
    border: none;
    padding: 10px 16px;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 600;
}

.btn-primary:hover:not(:disabled) {
    background: #0369a1;
}

.btn-accent {
    background: #d97706;
    color: #fff;
    border: none;
    padding: 12px 20px;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 600;
    width: 100%;
}

.btn-accent:hover:not(:disabled) {
    background: #b45309;
}

button:disabled {
    background: #475569 !important;
    color: #94a3b8 !important;
    cursor: not-allowed;
}

.migration-block {
    margin-top: 25px;
    padding-top: 20px;
    border-top: 1px solid #334155;
}

h4 {
    margin: 0 0 5px 0;
    color: #cbd5e1;
}

.footnote {
    font-size: 12px;
    color: #64748b;
    margin-bottom: 12px;
}

.status-banner {
    margin-top: 15px;
    padding: 10px;
    border-radius: 4px;
    font-size: 13px;
}

.success-banner {
    background: #064e3b;
    color: #34d399;
    border: 1px solid #059669;
}

.error-banner {
    background: #7f1d1d;
    color: #fca5a5;
    border: 1px solid #dc2626;
}

.catalog-table {
    width: 100%;
    border-collapse: collapse;
    margin-top: 15px;
    font-size: 14px;
}

th,
td {
    text-align: left;
    padding: 12px;
    border-bottom: 1px solid #334155;
}

th {
    color: #94a3b8;
    font-weight: 600;
}

.active-row {
    background: rgba(56, 189, 248, 0.08);
}

.db-name-cell {
    font-family: monospace;
    font-size: 15px;
}

.badge-indicator {
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: bold;
}

.badge-active {
    background: #064e3b;
    color: #34d399;
}

.badge-idle {
    background: #334155;
    color: #94a3b8;
}

.btn-small {
    background: #334155;
    color: #e2e8f0;
    border: 1px solid #475569;
    padding: 6px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
}

.btn-small:hover:not(:disabled) {
    background: #475569;
}

.empty-catalog {
    text-align: center;
    padding: 30px;
    color: #64748b;
    font-style: italic;
}
</style>