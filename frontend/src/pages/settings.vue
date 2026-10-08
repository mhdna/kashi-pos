<template>
  <div class="settings-page">
    <div class="settings-header">
      <v-text-field
        v-model="settingsSearch"
        class="settings-search mb-4"
        placeholder="Search settings…"
        prepend-inner-icon="mdi-magnify"
        variant="outlined"
        density="compact"
        clearable
        hide-details
      />

      <v-tabs v-model="tab" class="settings-tabs" color="primary" grow>
        <v-tab v-for="t in settingsTabs" :key="t.value" :value="t.value" class="settings-tab">
          <v-icon :icon="t.icon" size="20" class="settings-tab__icon" />
          <span class="settings-tab__label">{{ t.label }}</span>
        </v-tab>
      </v-tabs>
      <v-progress-linear v-if="settingsLoading" indeterminate color="primary" />
    </div>

    <div class="settings-scroll">
      <div v-if="normalizedSearch && !tabsWithMatches?.size" class="settings-empty">
        No settings match "{{ settingsSearch }}".
      </div>

      <v-window v-model="tab">
      <v-window-item value="general">
        <div class="settings-grid">
          <v-card v-show="panelMatches('general-appearance')" class="settings-card">
            <v-card-title>Appearance</v-card-title>
            <v-card-text>
              <v-switch
                :model-value="darkThemeEnabled"
                :loading="themeSaving"
                :disabled="themeSaving"
                label="Dark theme"
                color="primary"
                hide-details
                @update:model-value="setDarkTheme"
              />
              <v-switch
                :model-value="animationsEnabled"
                label="Animations"
                color="primary"
                hide-details
                @update:model-value="(value) => setAnimationsEnabled(value ?? false)"
              />
              <div class="text-body-2 mt-4">
                Quick access rows:
                <strong>{{ quickAccessRows === 0 ? "Hidden" : quickAccessRows }}</strong>
              </div>
              <v-slider
                :model-value="quickAccessRows"
                :min="0"
                :max="QUICK_ACCESS_MAX_ROWS"
                :step="1"
                color="primary"
                show-ticks="always"
                hint="Rows shown on the sale screen before it scrolls. 0 hides the panel."
                persistent-hint
                @update:model-value="setQuickAccessRows"
              />
              <div class="text-body-2 mt-4">
                Quick access text size: <strong>{{ quickAccessFontScale }}%</strong>
              </div>
              <v-slider
                :model-value="quickAccessFontScale"
                :min="QUICK_ACCESS_FONT_MIN"
                :max="QUICK_ACCESS_FONT_MAX"
                :step="10"
                color="primary"
                show-ticks="always"
                hide-details
                @update:model-value="setQuickAccessFontScale"
              >
                <template #append>
                  <v-btn
                    size="small"
                    variant="text"
                    :disabled="quickAccessFontScale === 100"
                    @click="setQuickAccessFontScale(100)"
                  >
                    Reset
                  </v-btn>
                </template>
              </v-slider>
              <v-switch
                :model-value="quickAccessShowCode"
                label="Show code on quick access cards"
                color="primary"
                hide-details
                @update:model-value="(value) => setQuickAccessShowCode(value ?? false)"
              />
              <v-switch
                :model-value="quickAccessShowName"
                label="Show name on quick access cards"
                color="primary"
                hide-details
                @update:model-value="(value) => setQuickAccessShowName(value ?? false)"
              />
              <v-switch
                :model-value="quickAccessShowRemove"
                label="Show remove (X) button on quick access cards"
                color="primary"
                hide-details
                @update:model-value="(value) => setQuickAccessShowRemove(value ?? false)"
              />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('general-quick-access-colors')" class="settings-card settings-card--wide">
            <v-card-title class="d-flex align-center">
              Quick access colors
              <v-spacer />
              <v-btn
                variant="tonal"
                size="small"
                prepend-icon="mdi-download"
                :loading="exportingQuickAccess"
                :disabled="!quickAccessItems.length"
                @click="exportQuickAccessCsv"
              >
                Export CSV
              </v-btn>
            </v-card-title>
            <v-card-text>
              <div class="d-flex align-center flex-wrap ga-3 mb-4">
                <span class="text-body-2">Cards come from</span>
                <v-btn-toggle
                  :model-value="quickAccessMode"
                  mandatory
                  density="compact"
                  variant="outlined"
                  color="primary"
                  :disabled="!auth.isAdmin || changingQuickAccessMode"
                  @update:model-value="changeQuickAccessMode"
                >
                  <v-btn size="small" value="till">This till</v-btn>
                  <v-btn size="small" value="kashi">kashi</v-btn>
                </v-btn-toggle>
                <span v-if="quickAccessMode === 'kashi'" class="text-caption text-medium-emphasis">
                  This register shows its branch's list from kashi. Edit cards and colours there (Branch settings →
                  Quick access).
                </span>
                <span v-else-if="!quickAccessKashiAvailable" class="text-caption text-medium-emphasis">
                  kashi isn't connected, or doesn't manage Quick Access yet.
                </span>
              </div>
              <div v-if="!quickAccessItems.length" class="text-body-2 text-medium-emphasis">
                {{
                  quickAccessMode === "kashi"
                    ? "No Quick Access cards set in kashi for this branch yet."
                    : "No quick access items yet. Pin products from the sale screen first."
                }}
              </div>
              <div v-for="item in quickAccessItems" :key="item.id" class="qa-color-row">
                <div class="qa-color-row__preview text-caption" :style="quickAccessCardStyle(item)">
                  <div class="text-truncate">{{ item.code }}</div>
                  <div class="text-truncate font-weight-medium">{{ item.name }}</div>
                  <div class="qa-color-row__price">${{ Number(item.price).toFixed(2) }}</div>
                </div>
                <div v-if="quickAccessMode !== 'kashi'" class="qa-color-row__pickers">
                  <div class="qa-color-row__swatches">
                    <span class="qa-color-row__label text-caption">Card</span>
                    <button
                      type="button"
                      class="qa-swatch qa-swatch--none"
                      :class="{ 'qa-swatch--selected': !item.quickAccessColor }"
                      title="No color"
                      @click="setQuickAccessItemColor(item, { quickAccessColor: '' })"
                    />
                    <button
                      v-for="color in QUICK_ACCESS_COLORS"
                      :key="color"
                      type="button"
                      class="qa-swatch"
                      :class="{ 'qa-swatch--selected': item.quickAccessColor === color }"
                      :style="{ backgroundColor: color }"
                      :title="color"
                      @click="setQuickAccessItemColor(item, { quickAccessColor: color })"
                    />
                  </div>
                  <div class="qa-color-row__swatches">
                    <span class="qa-color-row__label text-caption">Text</span>
                    <button
                      type="button"
                      class="qa-swatch qa-swatch--auto text-caption"
                      :class="{ 'qa-swatch--selected': !item.quickAccessTextColor }"
                      title="Automatic"
                      @click="setQuickAccessItemColor(item, { quickAccessTextColor: '' })"
                    >
                      A
                    </button>
                    <button
                      v-for="color in QUICK_ACCESS_TEXT_COLORS"
                      :key="color"
                      type="button"
                      class="qa-swatch"
                      :class="{ 'qa-swatch--selected': item.quickAccessTextColor === color }"
                      :style="{ backgroundColor: color }"
                      :title="color"
                      @click="setQuickAccessItemColor(item, { quickAccessTextColor: color })"
                    />
                  </div>
                </div>
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('general-branch')" class="settings-card">
            <v-card-title>Branch</v-card-title>
            <v-card-text class="field-grid">
              <v-text-field v-model="form.branchName" placeholder="Branch name" variant="outlined" density="compact" hide-details />
              <v-text-field
                v-model="form.branchCode"
                placeholder="Branch code"
                maxlength="8"
                variant="outlined"
                density="compact"
                hide-details
                @update:model-value="form.branchCode = form.branchCode.toUpperCase()"
              />
              <v-select
                v-model.number="form.salesInventoryId"
                :items="inventories"
                item-title="name"
                item-value="id"
                placeholder="Sales inventory"
                variant="outlined"
                density="compact"
                hide-details
                class="field-full"
              />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('general-database')" class="settings-card">
            <v-card-title>Database</v-card-title>
            <v-card-text>
              <v-text-field
                :model-value="databasePath"
                label="Current database"
                variant="outlined"
                density="compact"
                readonly
                hide-details
              />
              <div class="d-flex justify-end mt-2">
                <v-btn variant="text" color="primary" :loading="databaseSwitching" @click="chooseDatabase">
                  <v-icon start icon="mdi-folder-open-outline" /> Choose database
                </v-btn>
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('general-branches')" class="settings-card">
            <v-card-title>Branches</v-card-title>
            <v-card-text>
              <div class="text-caption text-medium-emphasis mb-3">
                Every branch set up on this machine has its own local database. Switching applies
                immediately, without restarting the app.
              </div>
              <v-select
                v-model="selectedBranchCode"
                :items="branchOptions"
                item-title="title"
                item-value="code"
                label="Active branch"
                variant="outlined"
                density="compact"
                hide-details
                :loading="branchSwitching"
                @update:model-value="switchBranch"
              />
              <div class="d-flex align-center justify-space-between mt-2">
                <span class="text-caption text-medium-emphasis">
                  Default at next launch: {{ defaultBranchCode || "—" }}
                </span>
                <v-btn
                  variant="text"
                  size="small"
                  :disabled="!activeBranchCode || activeBranchCode === defaultBranchCode"
                  :loading="settingDefaultBranch"
                  @click="setDefaultBranch(activeBranchCode)"
                >
                  Make active branch default
                </v-btn>
              </div>
              <v-divider class="my-4" />
              <div class="text-subtitle-2 mb-2">Add another branch</div>
              <v-text-field
                v-model="newBranch.name"
                label="Branch name"
                variant="outlined"
                density="compact"
                hide-details
                class="mb-2"
              />
              <v-text-field
                v-model="newBranch.code"
                label="Branch code"
                hint="Letters and numbers only"
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
                @update:model-value="newBranch.code = newBranch.code.toUpperCase()"
              />
              <v-text-field
                v-model="newBranch.apiKey"
                label="Central API key"
                variant="outlined"
                density="compact"
                hide-details
                class="mb-2"
              />
              <v-text-field
                v-model="newBranch.centralApiUrl"
                label="Central server URL"
                variant="outlined"
                density="compact"
                hide-details
                class="mb-2"
              />
              <div class="d-flex justify-end">
                <v-btn color="primary" variant="flat" :loading="creatingBranch" @click="createBranch">
                  Create branch
                </v-btn>
              </div>
              <v-alert v-if="branchActionError" type="error" density="compact" class="mt-3">
                {{ branchActionError }}
              </v-alert>

              <v-divider class="my-4" />
              <div class="text-subtitle-2 mb-2">Import branch history from kashi</div>
              <div class="text-caption text-medium-emphasis mb-2">
                If this branch already has real invoices, expenses, loans and closed shifts on
                kashi (e.g. this is a re-provisioned till), pull them down. Only runs on a branch
                with no local invoices, expenses, loans or shifts yet -- best-effort: line items
                or payment accounts kashi can no longer match exactly are reconstructed with a
                placeholder, not silently dropped.
              </div>
              <v-btn
                variant="outlined"
                :loading="importingHistory"
                @click="importBranchHistory"
              >
                Import branch history
              </v-btn>
              <v-alert v-if="historyImportResult" type="success" density="compact" class="mt-3">
                Imported {{ historyImportResult.invoices }} invoice(s),
                {{ historyImportResult.expenses }} expense(s), {{ historyImportResult.loans }}
                loan(s) and {{ historyImportResult.shiftCloses }} shift close(s).
                <span v-if="historyImportResult.unresolvedItems">
                  {{ historyImportResult.unresolvedItems }} line item(s) used a placeholder
                  product.
                </span>
              </v-alert>
              <v-alert v-if="historyImportError" type="error" density="compact" class="mt-3">
                {{ historyImportError }}
              </v-alert>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('general-sync')" class="settings-card">
            <v-card-title>Central sync (kashi)</v-card-title>
            <v-card-text class="field-grid">
              <v-text-field
                v-model="form.apiKey"
                placeholder="API key"
                variant="outlined"
                density="compact"
                hide-details
                :append-inner-icon="showApiKey ? 'mdi-eye' : 'mdi-eye-off'"
                :type="showApiKey ? 'text' : 'password'"
                @click:append-inner="showApiKey = !showApiKey"
              />
              <v-text-field
                v-model="form.centralApiUrl"
                placeholder="Central server URL (e.g. http://localhost:8080)"
                variant="outlined"
                density="compact"
                hide-details
                class="field-full"
              />
              <v-switch
                :model-value="form.closeShiftRequiresSync"
                :loading="closeShiftRequiresSyncSaving"
                label="Upload everything before closing a shift"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setCloseShiftRequiresSync(value)"
              />
              <div class="text-caption text-medium-emphasis field-full">
                Closing a shift first uploads every sale, return, expense and the rest still
                waiting for the office, showing progress and time left, and only then closes
                the shift and prints its receipt. Offline, a manager can override with the
                admin password.
              </div>

              <div class="field-full sync-everything">
                <div class="text-caption text-medium-emphasis mb-1">
                  Push every local client and invoice this server is missing, and
                  pull its catalog down. Safe to run again, and after changing the
                  server URL above.
                </div>
                <v-checkbox
                  v-model="resyncApplyStock"
                  density="compact"
                  hide-details
                  label="Also apply stock movements to kashi's inventory (only if kashi's stock was set up before these sales)"
                />
                <div class="d-flex align-center ga-2 mt-2">
                  <v-btn
                    color="primary"
                    variant="flat"
                    :loading="resyncRunning"
                    @click="runResync(false)"
                  >
                    Sync everything now
                  </v-btn>
                  <v-btn
                    variant="text"
                    :disabled="resyncRunning"
                    @click="runResync(true)"
                  >
                    Preview
                  </v-btn>
                  <span v-if="resyncProgress" class="text-caption text-medium-emphasis">
                    {{ resyncProgress }}
                  </span>
                </div>

                <v-alert
                  v-if="resyncError"
                  type="error"
                  density="compact"
                  variant="tonal"
                  class="mt-2"
                >
                  {{ resyncErrorFriendly || resyncError }}
                  <details v-if="resyncErrorFriendly && resyncErrorFriendly !== resyncError" class="mt-1">
                    <summary class="text-caption" style="cursor: pointer">Technical details</summary>
                    <div class="text-caption">{{ resyncError }}</div>
                  </details>
                </v-alert>

                <v-table v-if="resyncReport" density="compact" class="mt-3">
                  <thead>
                    <tr>
                      <th>Entity</th><th class="text-right">Local</th>
                      <th class="text-right">On kashi</th><th class="text-right">Missing</th>
                      <th class="text-right">{{ resyncReport.dryRun ? "Would queue" : "Queued" }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="row in resyncReport.entities" :key="row.entity">
                      <td>{{ row.entity }}</td>
                      <td class="text-right">{{ row.local }}</td>
                      <td class="text-right">{{ row.central }}</td>
                      <td class="text-right">{{ row.missing }}</td>
                      <td class="text-right">{{ row.queued }}</td>
                    </tr>
                  </tbody>
                </v-table>

                <v-table v-if="resyncReport?.fullCheck" density="compact" class="mt-3">
                  <thead>
                    <tr>
                      <th>Full check</th><th class="text-right">On kashi</th>
                      <th class="text-right">Fetched</th><th class="text-right">Deactivated</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="row in resyncReport.fullCheck.entities" :key="row.entity">
                      <td>{{ row.entity }}</td>
                      <td class="text-right">{{ row.central }}</td>
                      <td class="text-right">{{ row.fetched }}</td>
                      <td class="text-right">{{ row.deactivated }}</td>
                    </tr>
                  </tbody>
                </v-table>
              </div>
            </v-card-text>
          </v-card>

          <!-- Read-only: targets are set in kashi's admin UI only, this
               just confirms what's synced down for this branch. -->
          <v-card v-show="panelMatches('general-targets')" class="settings-card">
            <v-card-title>Targets</v-card-title>
            <v-card-text>
              <div v-if="!branchTargets.length" class="text-caption text-medium-emphasis">
                No targets set for this branch.
              </div>
              <v-table v-else density="compact">
                <thead>
                  <tr><th /><th>From</th><th>To</th><th>Progress</th></tr>
                </thead>
                <tbody>
                  <tr v-for="target in branchTargets" :key="target.id">
                    <td><span class="target-color-swatch" :style="{ background: target.color }" /></td>
                    <td>{{ new Date(target.dateFrom).toLocaleDateString() }}</td>
                    <td>{{ new Date(target.dateTo).toLocaleDateString() }}</td>
                    <td>{{ formatUsd(target.achievedCents / 100) }} / {{ formatUsd(target.targetAmountCents / 100) }}</td>
                  </tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <!-- kashi-admin (legacy): the separate local CLI tool's own admin
               API, unrelated to central sync above. Slated for removal once
               kashi's remote-command channel covers everything it does. -->
          <v-card v-show="panelMatches('general-admin-legacy')" class="settings-card">
            <v-card-title>kashi-admin (legacy)</v-card-title>
            <v-card-text class="field-grid">
              <v-text-field v-model="form.adminListen" placeholder="Listen address" variant="outlined" density="compact" hide-details />
              <v-text-field
                v-model="form.adminToken"
                placeholder="Admin token"
                variant="outlined"
                density="compact"
                hide-details
                :type="showAdminToken ? 'text' : 'password'"
                :append-inner-icon="showAdminToken ? 'mdi-eye' : 'mdi-eye-off'"
                @click:append-inner="showAdminToken = !showAdminToken"
              />
              <v-switch v-model="form.adminEnabled" label="Enable remote admin API" color="primary" hide-details class="field-full" />
            </v-card-text>
          </v-card>
        </div>
        <div class="settings-page-actions">
          <v-btn color="primary" variant="flat" :loading="saving || adminSettingsSaving" @click="saveGeneralSettings">
            <v-icon start icon="mdi-content-save-outline" /> Save general settings
          </v-btn>
        </div>
      </v-window-item>

      <v-window-item value="attendance">
        <div class="settings-grid">
          <v-card v-show="panelMatches('attendance-machine')" class="settings-card">
            <v-card-title>Attendance machine</v-card-title>
            <v-card-text class="field-grid">
              <v-text-field v-model="form.akuvoxIP" placeholder="Akuvox IP address" variant="outlined" density="compact" hide-details />
              <v-text-field v-model="form.akuvoxUsername" placeholder="Device username" variant="outlined" density="compact" hide-details />
              <v-text-field v-model="form.akuvoxPassword" placeholder="Device password" type="password" variant="outlined" density="compact" hide-details />
              <v-text-field v-model="form.attendanceApproverPassword" placeholder="Manager password" type="password" variant="outlined" density="compact" hide-details />
              <v-text-field v-model.number="form.attendanceDuplicateIntervalSeconds" placeholder="Duplicate interval (sec)" type="number" min="0" step="1" variant="outlined" density="compact" hide-details />
              <v-switch v-model="form.attendanceEnabled" label="Enable attendance" color="primary" hide-details class="field-full" />
              <v-switch v-model="form.attendanceCashierHistory" label="Cashiers can browse previous days" color="primary" hide-details class="field-full" />
              <v-switch
                v-model="form.autoCloseStaleShiftsEnabled"
                label="Auto-close previous-day shifts"
                color="primary"
                hide-details
                class="field-full"
              />
            </v-card-text>
          </v-card>
        </div>
        <div class="settings-page-actions">
          <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
            <v-icon start icon="mdi-content-save-outline" /> Save attendance settings
          </v-btn>
        </div>
      </v-window-item>

      <v-window-item value="printing">
        <div class="settings-grid">
          <v-card v-show="panelMatches('printing-printer')" class="settings-card">
            <v-card-title>Printer &amp; customer screen</v-card-title>
            <v-card-text class="field-grid">
              <v-select v-model="form.receiptPrinter" :items="printers" placeholder="Receipt printer" variant="outlined" density="compact" hide-details />
              <v-select v-model="form.printerSize" :items="['58mm', '80mm']" placeholder="Printing size" variant="outlined" density="compact" hide-details />
              <v-text-field v-model.number="form.receiptWidth" placeholder="Width (mm)" type="number" variant="outlined" density="compact" hide-details />
              <v-text-field v-model.number="form.receiptHeight" placeholder="Height (mm)" type="number" variant="outlined" density="compact" hide-details />
              <v-select v-model="form.screenPort" :items="serialPorts" placeholder="Customer screen COM port" variant="outlined" density="compact" hide-details />
              <v-switch v-model="form.receiptEnabled" label="Print receipts after checkout" color="primary" hide-details class="field-full" />
              <v-switch
                :model-value="form.shiftReceiptUploadMode !== 0"
                :loading="shiftReceiptUploadModeSaving"
                label="Show receipt-photo QR code after closing a shift"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setShiftReceiptUploadMode(value ? 2 : 0)"
              />
              <template v-if="form.shiftReceiptUploadMode !== 0">
                <v-select
                  :model-value="form.shiftReceiptUploadMode"
                  :items="shiftReceiptUploadModes"
                  :loading="shiftReceiptUploadModeSaving"
                  label="Close-shift receipt photo"
                  class="field-full"
                  variant="outlined"
                  density="compact"
                  hide-details
                  @update:model-value="(value) => setShiftReceiptUploadMode(value)"
                />
                <div class="text-caption text-medium-emphasis field-full">
                  After a shift closes, the cashier is shown a QR code to upload one photo of the
                  printed receipt. <b>Hybrid</b> sends it to the office when there's a connection
                  and buffers it on this machine otherwise; <b>Local</b> always uploads over the
                  shop Wi-Fi to this machine; <b>Central</b> always goes straight to the office.
                </div>
              </template>
              <div v-else class="text-caption text-medium-emphasis field-full">
                When off, closing a shift logs straight out without asking for a receipt photo.
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('printing-receipt')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-printer-outline" size="20" />
              Receipt settings
            </v-card-title>
            <v-card-text class="field-grid">
              <v-text-field v-model="form.marketName" placeholder="Market name" variant="outlined" density="compact" hide-details />
              <v-text-field v-model="form.marketPhone" placeholder="Market phone" variant="outlined" density="compact" hide-details />
              <v-text-field v-model="form.marketDescription" placeholder="Tagline" variant="outlined" density="compact" hide-details />
              <v-text-field v-model="form.branchPhone" placeholder="Branch phone" variant="outlined" density="compact" hide-details />
              <v-text-field v-model="form.registerName" placeholder="Register name (POS-01)" variant="outlined" density="compact" hide-details />
              <v-text-field v-model="form.website" placeholder="Website" variant="outlined" density="compact" hide-details />
              <v-select v-model="form.receiptFont" :items="['Belleza', 'Helvetica', 'Times', 'Courier']" placeholder="Title font" variant="outlined" density="compact" hide-details />
              <v-select v-model="form.receiptBodyFont" :items="['Printer Font A', 'Courier', 'Helvetica', 'Times', 'Belleza']" placeholder="Body font" variant="outlined" density="compact" hide-details />
              <v-text-field v-model.number="form.receiptTitleSize" placeholder="Market name size" type="number" min="8" max="40" variant="outlined" density="compact" hide-details />
              <v-text-field v-model.number="form.receiptBodySize" placeholder="Body size" type="number" min="6" max="24" variant="outlined" density="compact" hide-details />

              <v-textarea v-model="form.returnPolicy" placeholder="Return policy" rows="2" variant="outlined" density="compact" hide-details class="field-full" />

              <v-select
                v-model="form.socialPlatforms"
                :items="socialPlatformOptions"
                placeholder="Social platforms"
                multiple
                chips
                closable-chips
                hide-selected
                variant="outlined"
                density="compact"
                hide-details
                class="field-full"
              >
                <template #chip="{ props, item }">
                  <v-chip v-bind="props" :icon="platformIcon(String(item.value))" label variant="tonal" />
                </template>
                <template #item="{ props, item }">
                  <v-list-item v-bind="props" :prepend-icon="platformIcon(String(item.value))">
                    <template #title>
                      <span class="sr-only">{{ platformLabel(String(item.value)) }}</span>
                    </template>
                  </v-list-item>
                </template>
              </v-select>
              <v-text-field
                v-for="platform in form.socialPlatforms"
                :key="platform"
                v-model="form.socialHandles[platform]"
                :placeholder="`${platformLabel(platform)} handle`"
                :prepend-inner-icon="platformIcon(platform)"
                variant="outlined"
                density="compact"
                hide-details
              />

              <v-switch v-model="form.receiptCutoff" label="Cut paper after printing" color="primary" hide-details class="field-full" />
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
              <v-btn
                variant="tonal"
                color="primary"
                :loading="samplePrinting"
                @click="printCloseShiftSample"
              >
                <v-icon start icon="mdi-printer-eye" /> Print sample
              </v-btn>
            </v-card-actions>
          </v-card>
        </div>
        <div class="settings-page-actions">
          <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
            <v-icon start icon="mdi-content-save-outline" /> Save printing settings
          </v-btn>
        </div>
      </v-window-item>

      <v-window-item value="money">
        <div class="settings-grid">
          <v-card v-show="panelMatches('money-general')" class="settings-card">
            <v-card-title>Money settings</v-card-title>
            <v-card-text class="field-grid">
              <v-text-field v-model.number="form.taxRate" placeholder="VAT rate" type="number" step="0.01" min="0" max="1" variant="outlined" density="compact" hide-details />
              <v-text-field v-model.number="form.exchangeRate" placeholder="Legacy exchange rate" type="number" min="1" step="1" variant="outlined" density="compact" hide-details />
              <v-select v-model="form.roundingMode" :items="roundingModes" placeholder="Money rounding" variant="outlined" density="compact" hide-details />
              <v-select v-model="form.roundingCurrency" :items="roundingCurrencies" placeholder="Round by currency" variant="outlined" density="compact" hide-details />
              <v-select v-model="form.splitPrimaryCurrency" :items="roundingCurrencies" placeholder="Split primary currency" variant="outlined" density="compact" hide-details />
              <v-switch v-model="form.splitPrimaryFirst" color="primary" density="compact" hide-details :label="form.splitPrimaryFirst ? 'Primary currency first' : 'Rounding currency first'" />
              <v-text-field
                v-model.number="form.splitPrimaryStep"
                placeholder="Split step (0 = use currency's cash rounding)"
                type="number"
                min="0"
                step="1"
                variant="outlined"
                density="compact"
                hide-details
              />
              <v-select
                :model-value="calculatorTotalCurrency"
                :items="calculatorDisplayCurrencies"
                placeholder="Calculator total currency"
                variant="outlined"
                density="compact"
                hide-details
                @update:model-value="(value) => setCalculatorTotalCurrency(value ?? 'USD')"
              />
              <v-text-field
                v-model.number="form.maxOverpayment"
                placeholder="Max overpayment (USD)"
                type="number"
                min="0"
                step="0.01"
                :disabled="!form.allowOverpayment"
                variant="outlined"
                density="compact"
                hide-details
              />
              <v-switch v-model="form.allowOverpayment" label="Allow overpayment" color="primary" hide-details class="field-full" />
              <v-switch
                :model-value="form.autoAppendLbpZeros"
                :loading="autoAppendLbpZerosSaving"
                label="Auto-append 000 to LBP amounts"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setAutoAppendLbpZeros(value)"
              />
              <v-switch
                :model-value="form.overchangeConfirmationEnabled"
                :loading="overchangeConfirmationSaving"
                label="Confirm when giving more change than owed"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setOverchangeConfirmationEnabled(value)"
              />
              <div class="text-caption text-medium-emphasis field-full">
                Shows a "Confirm change amount" prompt in the checkout calculator when the
                cashier enters change or a refund larger than what's owed back. Turn off to
                accept it straight away.
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-float')" class="settings-card">
            <v-card-title>Opening cash float</v-card-title>
            <v-card-text class="field-grid">
              <v-switch
                :model-value="form.openingFloatEnabled"
                :loading="openingFloatEnabledSaving"
                label="Enable opening cash float"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setOpeningFloatEnabled(value)"
              />
              <div class="text-caption text-medium-emphasis field-full">
                When off, shifts start with no opening float, the "Set opening cash" controls
                on the Shifts page are hidden, and the opening amount is left out of every
                balance, income total and close-shift receipt.
              </div>
              <template v-if="form.openingFloatEnabled">
                <v-text-field v-model.number="form.defaultOpeningFloatUsd" placeholder="Opening float (USD)" type="number" min="0" step="0.01" variant="outlined" density="compact" hide-details />
                <v-text-field v-model.number="form.defaultOpeningFloatLbp" placeholder="Opening float (LBP)" type="number" min="0" step="1" variant="outlined" density="compact" hide-details />
                <v-alert v-if="openingFloatNegative" type="warning" variant="tonal" density="compact" class="field-full">
                  Opening float cannot be negative.
                </v-alert>
                <v-btn color="primary" variant="tonal" :disabled="openingFloatNegative" class="field-full" @click="openingFloatDialog = true">Save opening float</v-btn>
                <v-switch
                  :model-value="form.countOpeningCashOnClose"
                  :loading="countOpeningCashOnCloseSaving"
                  label="Count opening float in close-shift cash count"
                  color="primary"
                  hide-details
                  class="field-full"
                  @update:model-value="(value) => setCountOpeningCashOnClose(value)"
                />
                <div class="text-caption text-medium-emphasis field-full">
                  When off, the cashier enters only the cash taken in during the shift; the
                  opening float is left out of the expected total and the printed close-shift
                  reconciliation.
                </div>
              </template>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-exchange')" class="settings-card">
            <v-card-title>Exchange settings</v-card-title>
            <v-card-text class="field-grid">
              <v-text-field v-model.number="form.exchangeWindowDays" label="Days" type="number" min="0" step="1" variant="outlined" density="compact" hide-details />
              <v-text-field v-model.number="form.exchangeWindowHours" label="Hours" type="number" min="0" max="23" step="1" variant="outlined" density="compact" hide-details />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-currencies')" class="settings-card settings-card--wide">
            <v-card-title>Currencies &amp; rates</v-card-title>
            <v-card-text>
              <v-alert type="info" variant="tonal" density="compact" class="mb-4">
                Currencies and their rates are set in kashi and sync down here.
              </v-alert>
              <v-table density="compact">
                <thead>
                  <tr>
                    <th>Code</th>
                    <th>Name</th>
                    <th>Symbol</th>
                    <th>Units per USD</th>
                    <th>Cash rounding unit</th>
                    <th>Active</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="currency in currencies" :key="currency.code">
                    <td>{{ currency.code }}</td>
                    <td>{{ currency.name }}</td>
                    <td>{{ currency.symbol }}</td>
                    <td>{{ currency.unitsPerUsd }}</td>
                    <td>{{ currency.cashRoundingUnit }}</td>
                    <td>{{ currency.isActive ? "Yes" : "No" }}</td>
                  </tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-payment-methods')" class="settings-card settings-card--wide">
            <v-card-title>Payment methods</v-card-title>
            <v-card-text>
              <v-alert type="info" variant="tonal" density="compact" class="mb-4">
                Payment methods are set in kashi and sync down here. One kashi
                no longer has stops being offered at checkout; past invoices
                that used it still open.
              </v-alert>
              <v-table density="compact">
                <thead>
                  <tr>
                    <th>Order</th>
                    <th>Name</th>
                    <th>Currency</th>
                    <th>Color</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(method, index) in orderablePaymentMethods" :key="method.id">
                    <td>{{ index + 1 }}</td>
                    <td>{{ method.name }}</td>
                    <td>{{ method.currencyCode }}</td>
                    <td>
                      <span
                        class="payment-method-color-swatch"
                        :style="{ background: paymentAccentColor(method) }"
                      />
                    </td>
                  </tr>
                  <tr v-if="!orderablePaymentMethods.length">
                    <td colspan="4" class="text-medium-emphasis">
                      No payment methods have synced yet.
                    </td>
                  </tr>
                </tbody>
              </v-table>
              <v-switch
                :model-value="!form.hidePaymentMethodCurrency"
                :loading="paymentMethodSaving"
                color="primary"
                hide-details
                label="Show currency on payment method buttons"
                class="mt-2"
                @update:model-value="(value) => setHidePaymentMethodCurrency(!value)"
              />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-price-lists')" class="settings-card">
            <v-card-title>Price lists</v-card-title>
            <v-card-text>
              <v-alert type="info" variant="tonal" density="compact" class="mb-4">
                Prices are set in kashi and sync down here. A product that no
                active list prices cannot be sold.
              </v-alert>
              <v-table density="compact">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Active</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="list in priceLists" :key="list.id">
                    <td>
                      {{ list.name }}
                      <v-chip v-if="list.isDefault" size="x-small" class="ml-2">default</v-chip>
                    </td>
                    <td>{{ list.isActive ? "Yes" : "No" }}</td>
                    <td>
                      <v-btn icon="mdi-format-list-bulleted-square" variant="text" size="small" title="View items" @click="openManageList('price', list)" />
                    </td>
                  </tr>
                  <tr v-if="!priceLists.length">
                    <td colspan="3" class="text-medium-emphasis">No price lists have synced yet.</td>
                  </tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-discount-lists')" class="settings-card">
            <v-card-title>Discount lists</v-card-title>
            <v-card-text>
              <v-alert type="info" variant="tonal" density="compact" class="mb-4">
                Discounts are set in kashi and sync down here.
              </v-alert>
              <v-table density="compact">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Active</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="list in discountLists" :key="list.id">
                    <td>
                      {{ list.name }}
                      <v-chip v-if="list.isDefault" size="x-small" class="ml-2">default</v-chip>
                    </td>
                    <td>{{ list.isActive ? "Yes" : "No" }}</td>
                    <td>
                      <v-btn icon="mdi-format-list-bulleted-square" variant="text" size="small" title="View items" @click="openManageList('discount', list)" />
                    </td>
                  </tr>
                  <tr v-if="!discountLists.length">
                    <td colspan="3" class="text-medium-emphasis">No discount lists have synced yet.</td>
                  </tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>
        </div>
        <div class="settings-page-actions">
          <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
            <v-icon start icon="mdi-content-save-outline" /> Save money settings
          </v-btn>
        </div>
      </v-window-item>

      <v-window-item value="coupons">
        <div class="settings-grid">
          <v-card v-show="panelMatches('coupons-types')" class="settings-card">
            <v-card-title>Coupon types</v-card-title>
            <v-card-text>
              <div class="d-flex flex-wrap ga-2 align-start">
                <v-text-field v-model="couponTypeForm.name" placeholder="Type name" variant="outlined" density="compact" style="max-width: 160px" />
                <v-text-field v-model.number="couponTypeForm.value" placeholder="Value" type="number" min="0.01" step="0.01" variant="outlined" density="compact" style="max-width: 110px" />
                <v-text-field v-model="couponTypeForm.currency" placeholder="Currency" maxlength="3" variant="outlined" density="compact" style="max-width: 100px" />
                <v-btn color="primary" :loading="couponTypeSaving" @click="saveCouponType">{{ editingCouponTypeId ? "Save" : "Add" }}</v-btn>
                <v-btn v-if="editingCouponTypeId" variant="text" @click="cancelCouponTypeEdit">Cancel</v-btn>
              </div>
              <div class="d-flex align-center ga-4 mb-2">
                <v-checkbox
                  v-model="couponTypeForm.codeRequired"
                  density="compact"
                  hide-details
                  label="Code required"
                  title="Uncheck for coupon types that need no code or reference number at all - they're added directly at checkout"
                />
                <v-checkbox v-model="couponTypeForm.referenceOnly" :disabled="!couponTypeForm.codeRequired" density="compact" hide-details label="Reference-only" />
                <v-checkbox v-model="couponTypeForm.isActive" density="compact" hide-details label="Active" />
                <v-checkbox
                  v-model="couponTypeForm.calculateOnCheckout"
                  density="compact"
                  hide-details
                  label="Don't count in totals (settled later)"
                  title="Tick when the coupon is settled later (e.g. reimbursed by the issuer) instead of collected by the branch now. Its payments are left out of the shift's total and net sales."
                />
              </div>
              <v-list density="compact" bg-color="transparent">
                <v-list-item v-for="couponType in couponTypes" :key="couponType.id" :title="couponType.name" :subtitle="`${(Number(couponType.valueCents) / 100).toFixed(2)} ${couponType.currencyCode}${couponType.codeRequired === false ? ' · No code required' : couponType.referenceOnly ? ' · Reference-only' : ''}${couponType.calculateOnCheckout ? ' · Not counted in totals' : ''}`">
                  <template #append>
                    <v-btn icon="mdi-pencil-outline" variant="text" density="compact" title="Edit coupon type" @click="editCouponType(couponType)" />
                    <v-btn icon="mdi-delete-outline" variant="text" density="compact" title="Delete coupon type" @click="removeCouponType(couponType)" />
                  </template>
                </v-list-item>
                <v-list-item v-if="!couponTypes.length" title="No coupon types configured" />
              </v-list>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('coupons-issued')" class="settings-card">
            <v-card-title>Issued coupons</v-card-title>
            <v-card-text>
              <div class="d-flex flex-wrap ga-2 align-start mb-2">
                <v-text-field v-model="couponCodeForm" placeholder="New coupon code" variant="outlined" density="compact" />
                <v-select v-model="couponIssueTypeId" :items="couponTypes" item-title="name" item-value="id" placeholder="Type" variant="outlined" density="compact" />
                <v-btn color="secondary" :loading="couponIssuing" @click="issueCoupon">Issue</v-btn>
              </div>
              <v-list density="compact" bg-color="transparent">
                <v-list-item v-for="coupon in coupons" :key="coupon.code" :title="coupon.code" :subtitle="`${(Number(coupon.valueCents) / 100).toFixed(2)} · ${coupon.status}`">
                  <template #append><v-btn icon="mdi-delete-outline" variant="text" density="compact" title="Delete coupon" @click="removeCoupon(coupon)" /></template>
                </v-list-item>
                <v-list-item v-if="!coupons.length" title="No coupons issued" />
              </v-list>
            </v-card-text>
          </v-card>
        </div>
      </v-window-item>

      <v-window-item value="behavior">
        <div class="settings-grid">
          <v-card v-show="panelMatches('behavior-custom-pricing')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-sale-outline" size="20" />
              Custom item pricing
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.customItemPricesEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Custom item prices"
                @update:model-value="setCustomPricesEnabled"
              />
              <v-switch
                :model-value="form.perUnitItemPricesEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Per-unit price/discount editing"
                @update:model-value="setPerUnitPricesEnabled"
              />
              <v-switch
                :model-value="form.customItemDiscountsEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Custom product discounts"
                @update:model-value="setCustomDiscountsEnabled"
              />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('behavior-price-change-mode')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-tag-multiple-outline" size="20" />
              Price change mode
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.priceChangeManualOverrideEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Manual override (independent price & discount)"
                @update:model-value="setPriceChangeManualOverrideEnabled"
              />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('behavior-invoice-keyboard')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-keyboard" size="20" />
              Invoice keyboard mode
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.invoiceKeyboardModeEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Enable keyboard mode"
                @update:model-value="setInvoiceKeyboardModeEnabled"
              />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('behavior-product-search')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-magnify" size="20" />
              Product search button
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.productSearchEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Show the product search button on the sale screen"
                @update:model-value="setProductSearchEnabled"
              />
              <div class="text-caption text-medium-emphasis mt-2">
                When off, the magnifier button is hidden from the sale screen toolbar.
                This can also be changed remotely from kashi.
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('behavior-scan-by-code')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-barcode-scan" size="20" />
              Product code in scan field
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.scanByCodeEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Accept a product code in the scan field"
                @update:model-value="setScanByCodeEnabled"
              />
              <div class="text-caption text-medium-emphasis mt-2">
                When nothing has the entered barcode, it is tried as a product code. If several
                variants share that code, the cashier picks one. This can also be changed remotely
                from kashi.
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('behavior-client-required')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-account-off-outline" size="20" />
              Client on checkout
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.clientRequired"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Require selecting a client before checkout"
                @update:model-value="setClientRequired"
              />
              <div class="text-caption text-medium-emphasis mt-2">
                When off, invoices default to Anonymous automatically - cashiers can still
                enter a phone number and name to attach a real client.
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('behavior-salesperson-per-invoice')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-account-tie-outline" size="20" />
              Salesperson per invoice
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.salespersonPerInvoiceRequired"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Require picking a salesperson on every invoice"
                @update:model-value="setSalespersonPerInvoiceRequired"
              />
              <div class="text-caption text-medium-emphasis mt-2">
                When on, the salesperson selection is cleared for each new invoice so
                the cashier must choose again. When off, the previous invoice's
                salesperson carries forward. This can also be changed remotely from kashi.
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('behavior-non-admin-month-nav')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-calendar-month-outline" size="20" />
              Income chart month navigation
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.nonAdminMonthNavEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Let non-admins browse other months on the income chart"
                @update:model-value="setNonAdminMonthNavEnabled"
              />
              <div class="text-caption text-medium-emphasis mt-2">
                When off, only admins can use the back/forward buttons on the monthly
                income chart - non-admins only see the current month.
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('behavior-visitor-counter')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-account-group-outline" size="20" />
              People visits counter
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.visitorCounterEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Show the visits counter in the side menu"
                @update:model-value="setVisitorCounterEnabled"
              />
              <div class="text-caption text-medium-emphasis mt-2">
                Adds an up / down counter to the side menu for tallying how many people
                visit each day. The daily total also shows as an orange chip in the
                monthly shift income table. Turning this off only hides those - recorded
                counts are kept.
              </div>
            </v-card-text>
          </v-card>

        </div>
      </v-window-item>

      <v-window-item value="admin">
        <div class="settings-grid">
          <AppUpdateCard v-show="panelMatches('admin-app-updates')" />

          <v-card v-show="panelMatches('admin-page-access')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-lock-outline" size="20" />
              Page access
            </v-card-title>
            <v-card-text>
              <v-switch
                v-for="page in lockablePages"
                :key="page"
                :model-value="pageUnlocks[page] ?? false"
                :loading="pageUnlockSaving"
                color="primary"
                hide-details
                :label="pageUnlockLabels[page] ?? page"
                class="mb-1"
                @update:model-value="(value) => togglePageUnlock(page, value)"
              />
            </v-card-text>
          </v-card>

          <v-form v-show="panelMatches('admin-change-password')" class="settings-form-wrap" @submit.prevent="changePassword">
            <v-card class="settings-card">
              <v-card-title class="d-flex align-center ga-3">
                <v-icon icon="mdi-lock-reset" size="20" />
                Change admin password
              </v-card-title>
              <div class="text-caption text-medium-emphasis px-4 pb-2">
                The password entered on the login screen's admin sign-in.
              </div>
              <v-card-text>
                <v-text-field
                  v-model="passwordForm.current"
                  placeholder="Current admin password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                  :append-inner-icon="showPw ? 'mdi-eye' : 'mdi-eye-off'"
                  @click:append-inner="showPw = !showPw"
                />
                <v-text-field
                  v-model="passwordForm.newPass"
                  placeholder="New password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                />
                <v-text-field
                  v-model="passwordForm.confirm"
                  placeholder="Confirm new password"
                  variant="outlined"
                  density="compact"
                  :type="showPw ? 'text' : 'password'"
                  :rules="[(v) => v === passwordForm.newPass || 'Passwords do not match']"
                />
              </v-card-text>
              <v-card-actions>
                <v-spacer />
                <v-btn color="primary" variant="flat" type="submit" :loading="changingPassword">
                  <v-icon start icon="mdi-lock-reset" /> Change password
                </v-btn>
              </v-card-actions>
            </v-card>
          </v-form>

          <v-form v-show="panelMatches('admin-cashier-pin')" class="settings-form-wrap" @submit.prevent="saveCashierPin">
            <v-card class="settings-card">
              <v-card-title class="d-flex align-center ga-3">
                <v-icon icon="mdi-dialpad" size="20" />
                Cashier login PIN
              </v-card-title>
              <div class="text-caption text-medium-emphasis px-4 pb-2">
                The shared 4-digit PIN cashiers enter on the login keypad.
              </div>
              <v-card-text>
                <v-text-field
                  v-model="cashierPinForm.newPin"
                  placeholder="New 4-digit PIN"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  inputmode="numeric"
                  maxlength="4"
                  :type="showCashierPin ? 'text' : 'password'"
                  :append-inner-icon="showCashierPin ? 'mdi-eye' : 'mdi-eye-off'"
                  :hint="cashierPinCurrent ? `Current: ${cashierPinCurrent}` : ''"
                  persistent-hint
                  @click:append-inner="showCashierPin = !showCashierPin"
                />
                <v-text-field
                  v-model="cashierPinForm.confirm"
                  placeholder="Confirm PIN"
                  variant="outlined"
                  density="compact"
                  inputmode="numeric"
                  maxlength="4"
                  :type="showCashierPin ? 'text' : 'password'"
                  :rules="[(v) => v === cashierPinForm.newPin || 'PINs do not match']"
                />
              </v-card-text>
              <v-card-actions>
                <v-spacer />
                <v-btn
                  color="primary"
                  variant="flat"
                  type="submit"
                  :loading="savingCashierPin"
                  :disabled="!/^\d{4}$/.test(cashierPinForm.newPin) || cashierPinForm.newPin !== cashierPinForm.confirm"
                >
                  <v-icon start icon="mdi-dialpad" /> Update PIN
                </v-btn>
              </v-card-actions>
            </v-card>
          </v-form>

          <v-card v-show="panelMatches('admin-users')" class="settings-card settings-card--wide">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-account-group-outline" size="20" />
              Users
              <v-spacer />
              <v-btn color="primary" size="small" @click="openUserDialog()">
                <v-icon start icon="mdi-plus" /> Add user
              </v-btn>
            </v-card-title>
            <div class="text-caption text-medium-emphasis px-4 pb-2">
              Users synced from kashi are managed there. Locally you can only reset a
              forgotten PIN for them.
            </div>
            <v-data-table
              :headers="userHeaders"
              :items="users"
              items-per-page="-1"
              hide-default-footer
              density="comfortable"
              hover
            >
              <template #item.username="{ item }">
                {{ item.username }}
                <v-chip
                  v-if="item.isSynced"
                  class="ml-2"
                  color="primary"
                  size="x-small"
                  variant="tonal"
                >
                  kashi
                </v-chip>
              </template>
              <template #item.active="{ item }">
                <v-chip :color="item.active ? 'green' : 'red'" size="small" variant="tonal">
                  {{ item.active ? "Active" : "Disabled" }}
                </v-chip>
              </template>
              <template #item.actions="{ item }">
                <v-btn
                  v-if="item.isSynced"
                  icon
                  size="small"
                  variant="text"
                  title="Reset PIN"
                  @click="openUserDialog(item)"
                >
                  <v-icon icon="mdi-key-outline" size="18" />
                </v-btn>
                <template v-else>
                  <v-btn icon size="small" variant="text" @click="openUserDialog(item)">
                    <v-icon icon="mdi-pencil-outline" size="18" />
                  </v-btn>
                  <v-btn icon size="small" variant="text" @click="deleteUserConfirm(item)">
                    <v-icon icon="mdi-delete-outline" size="18" color="red" />
                  </v-btn>
                </template>
              </template>
            </v-data-table>
          </v-card>

          <v-card v-show="panelMatches('admin-salespersons')" class="settings-card settings-card--wide">
            <v-card-title class="d-flex align-center ga-3">
              <v-icon icon="mdi-account-tie" size="20" />
              Salespersons
              <v-spacer />
              <v-btn
                v-if="!salespersonsSyncManaged"
                color="primary"
                size="small"
                @click="openSalespersonDialog()"
              >
                <v-icon start icon="mdi-plus" /> Add salesperson
              </v-btn>
            </v-card-title>
            <div class="text-caption text-medium-emphasis px-4 pb-2">
              <template v-if="salespersonsSyncManaged">
                The roster (name, active state) is managed from kashi and synced here read-only.
                Only the attendance device ID can be set locally.
              </template>
              <template v-else>
                Salespersons are managed locally on this register.
              </template>
            </div>
            <v-data-table
              :headers="salespersonHeaders"
              :items="salespersons"
              items-per-page="-1"
              hide-default-footer
              density="comfortable"
              hover
            >
              <template #item.color="{ item }">
                <span class="salesperson-color-dot" :style="{ background: salespersonColor(item) }" />
              </template>
              <template #item.isActive="{ item }">
                <v-chip v-if="item.isSynced" :color="item.isActive ? 'green' : 'red'" size="small" variant="tonal">
                  {{ item.isActive ? "Active" : "Inactive" }}
                </v-chip>
                <span v-else class="text-caption text-medium-emphasis">Not synced</span>
              </template>
              <template #item.actions="{ item }">
                <v-btn icon size="small" variant="text" @click="openSalespersonDialog(item)">
                  <v-icon icon="mdi-pencil-outline" size="18" />
                </v-btn>
                <v-btn
                  v-if="!item.isSynced"
                  icon
                  size="small"
                  variant="text"
                  @click="deleteSalespersonConfirm(item)"
                >
                  <v-icon icon="mdi-delete-outline" size="18" color="red" />
                </v-btn>
              </template>
            </v-data-table>
          </v-card>
        </div>
      </v-window-item>
      </v-window>
    </div>

    <v-dialog v-model="userDialog" max-width="400">
      <v-card>
        <v-card-title>
          {{ editingSyncedUser ? "Reset PIN" : editingUser ? "Edit User" : "New User" }}
        </v-card-title>
        <v-card-text>
          <div v-if="editingSyncedUser" class="text-body-2 text-medium-emphasis mb-3">
            {{ editingUser.username }} is managed from kashi. Only the PIN can be reset here.
          </div>
          <v-text-field
            v-if="!editingSyncedUser"
            v-model="userForm.username"
            placeholder="Username"
            variant="outlined"
            density="compact"
            class="mb-2"
          />
          <v-text-field
            v-model="userForm.password"
            variant="outlined"
            density="compact"
            class="mb-2"
            :type="showUpw ? 'text' : 'password'"
            :append-inner-icon="showUpw ? 'mdi-eye' : 'mdi-eye-off'"
            @click:append-inner="showUpw = !showUpw"
            :placeholder="editingSyncedUser ? 'New PIN' : editingUser ? 'Leave blank to keep current' : 'Password'"
          />
          <v-select
            v-if="!editingSyncedUser"
            v-model="userForm.roleId"
            :items="roles"
            item-title="name"
            item-value="id"
            placeholder="Role"
            variant="outlined"
            density="compact"
            class="mb-2"
          />
          <v-text-field
            v-if="!editingSyncedUser"
            v-model="userForm.attendanceUserId"
            placeholder="Akuvox UserID (leave blank for visitors)"
            variant="outlined"
            density="compact"
          />
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="userDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn
            color="primary"
            variant="flat"
            :disabled="editingSyncedUser ? !userForm.password : !userForm.username"
            :loading="savingUser"
            @click="saveUser"
            >Save</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog :model-value="showDeactivationConfirm" max-width="480" persistent>
      <v-card v-if="pendingDeactivation">
        <v-card-title>Deactivate products no longer on kashi?</v-card-title>
        <v-card-text>
          <div class="mb-2">
            kashi no longer has the following. This will deactivate them here
            too (hidden from checkout, not deleted -- past invoices and shift
            reports still show them). This confirmation only appears once;
            future syncs will do this automatically.
          </div>
          <div v-if="pendingDeactivation.productBarcodes.length" class="mb-2">
            <div class="text-caption text-medium-emphasis">
              {{ pendingDeactivation.productBarcodes.length }} product(s)
            </div>
            <ul class="text-body-2">
              <li v-for="(name, i) in pendingDeactivation.productNames.slice(0, 10)" :key="pendingDeactivation.productBarcodes[i]">
                {{ name || pendingDeactivation.productBarcodes[i] }}
              </li>
              <li v-if="pendingDeactivation.productBarcodes.length > 10" class="text-medium-emphasis">
                …and {{ pendingDeactivation.productBarcodes.length - 10 }} more
              </li>
            </ul>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" :disabled="deactivationConfirming" @click="cancelDeactivation">Cancel</v-btn>
          <v-spacer />
          <v-btn color="primary" variant="flat" :loading="deactivationConfirming" @click="confirmDeactivationAndResync">
            Deactivate and sync
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="deleteDialog" max-width="400">
      <v-card>
        <v-card-title>Delete User</v-card-title>
        <v-card-text>Are you sure you want to delete {{ deleteTarget?.username }}?</v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="deleteDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn color="red" variant="flat" @click="deleteUser">Delete</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="salespersonDialog" max-width="400">
      <v-card>
        <v-card-title>
          {{
            !salespersonNameEditable
              ? "Attendance ID"
              : editingSalesperson
                ? "Edit Salesperson"
                : "New Salesperson"
          }}
        </v-card-title>
        <v-card-text>
          <v-text-field
            v-if="salespersonNameEditable"
            v-model="salespersonName"
            placeholder="Full name"
            variant="outlined"
            density="compact"
            autofocus
          />
          <div v-else class="text-body-2 text-medium-emphasis mb-3">
            {{ editingSalesperson?.fullName }} --
            {{
              editingSalesperson?.isSynced
                ? "name, color and active state are managed from kashi."
                : "not in kashi; add them there to manage their name."
            }}
          </div>
          <v-text-field v-model="salespersonAttendanceUserId" placeholder="Akuvox attendance ID" variant="outlined" density="compact" />
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="salespersonDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn
            color="primary"
            variant="flat"
            :disabled="salespersonNameEditable && !salespersonName.trim()"
            :loading="savingSalesperson"
            @click="saveSalesperson"
            >Save</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="deleteSalespersonDialog" max-width="400">
      <v-card>
        <v-card-title>Delete Salesperson</v-card-title>
        <v-card-text>Delete {{ deleteSalespersonTarget?.fullName }}?</v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="deleteSalespersonDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn color="red" variant="flat" @click="deleteSalesperson">Delete</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="manageListDialog" max-width="520">
      <v-card>
        <v-card-title>{{ manageListName }} items</v-card-title>
        <v-card-text>
          <v-table density="compact">
            <thead>
              <tr>
                <th>Product</th>
                <th>{{ manageListType === "price" ? "Price" : "Discount" }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in manageListItems" :key="item.id">
                <td>{{ item.productUid }}</td>
                <td>{{ manageListType === "price" ? `$${Number(item.price).toFixed(2)}` : `${item.discount}%` }}</td>
              </tr>
              <tr v-if="!manageListItems.length">
                <td colspan="2" class="text-medium-emphasis">No items yet.</td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="manageListDialog = false">Close</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="openingFloatDialog" max-width="380">
      <v-card>
        <v-card-title>Save opening float</v-card-title>
        <v-card-text>
          <div class="text-body-2 mb-3">
            Applies to shifts opened after saving. Requires the manager password.
          </div>
          <div class="text-caption text-medium-emphasis mb-1">Manager password</div>
          <v-text-field
            v-model="openingFloatPassword"
            placeholder="••••••••"
            type="password"
            variant="outlined"
            density="compact"
            autofocus
            @keyup.enter="confirmOpeningFloatSave"
          />
          <v-alert v-if="openingFloatError" type="error" variant="tonal" density="compact">{{ openingFloatError }}</v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="openingFloatDialog = false">Cancel</v-btn>
          <v-btn color="primary" :loading="openingFloatSaving" @click="confirmOpeningFloatSave">Save</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar.show" class="app-snackbar" timeout="3500" location="top" variant="flat">
      {{ snackbar.text }}
    </v-snackbar>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onBeforeUnmount, onMounted, watch } from "vue";
import {
  GetAppSettings,
  GetDatabaseStatus,
  ChooseDatabase,
  ListBranches,
  CreateBranch,
  SwitchBranch,
  SetDefaultBranch,
  PullBranchHistory,
  ListBranchTargetsWithProgress,
  UpdateSettings,
  UpdateCentralAPIURL,
  UpdateAttendanceConfig,
  UpdateAttendanceApproverPassword,
  SetAttendanceDuplicateInterval,
  SetExchangeWindowHours,
  SetSplitDisplay,
  SetCustomItemDiscountsEnabled,
  SetCustomItemPricesEnabled,
  SetPerUnitItemPricesEnabled,
  SetPriceChangeManualOverrideEnabled,
  SetInvoiceKeyboardModeEnabled,
  SetProductSearchEnabled,
  SetScanByCodeEnabled,
  SetVisitorCounterEnabled,
  SetClientRequired,
  SetSalespersonPerInvoiceRequired,
  SetNonAdminMonthNavEnabled,
  SetHidePaymentMethodCurrency,
  SetAutoAppendLbpZerosEnabled,
  SetOverchangeConfirmationEnabled,
  SetCloseShiftRequiresSyncEnabled,
  SetOpeningFloatEnabled,
  SetCountOpeningCashOnCloseEnabled,
  SetShiftReceiptUploadMode,
  SetPageUnlocked,
  UpdateDefaultOpeningFloat,
  SetAutoCloseStaleShiftsEnabled,
  SetAttendanceCashierHistoryEnabled,
  UpdateUserAttendanceID,
  UpdateSalespersonAttendanceID,
  UpdateReceiptSettings,
  ChangeAdminPassword,
  GetCashierPIN,
  SetCashierPIN,
  ListUsers,
  CreateUser,
  UpdateUser,
  DeleteUser,
  ListRoles,
  ListSerialPorts,
  ListPrinters,
  PrintCurrentCloseShiftSample,
    ListInventories,
  ListCouponTypes,
  ListCoupons,
  CreateCouponType,
  UpdateCouponType,
  CreateCoupon,
  DeleteCoupon,
  DeleteCouponType,
  ListSalespersons,
  CreateSalesperson,
  UpdateSalesperson,
  DeleteSalesperson,
  ListCurrencies,
  ListPaymentMethods,
  SetTheme,
  ListPriceLists,
  ListPriceListItems,
  ListDiscountLists,
  ListDiscountListItems,
  GetSyncStatus,
  ListQuickAccessProducts,
  GetQuickAccessSource,
  SetQuickAccessMode,
  SetQuickAccessColor,
  ExportQuickAccessCSV,
} from "../../wailsjs/go/main/App";
import { EventsOn, WindowReloadApp } from "../../wailsjs/runtime/runtime";
import { formatUsd } from "@/lib/format";
import { isSpecialPaymentMethod } from "@/lib/paymentMethods";
import { friendlySyncErrorReason } from "@/lib/syncErrors";
import { paymentAccentColor } from "@/lib/paymentColor";
import { useAuthStore } from "@/stores/auth";
import { useResyncStore } from "@/stores/resync";
import { useClient } from "@/composables/useClient";
import { useSalesperson } from "@/composables/useSalesperson";
import { salespersonColor } from "@/lib/salespersonColor";
import { usePageUnlocks, lockablePages } from "@/composables/usePageUnlocks";
import { useVisitorCounter } from "@/composables/useVisitorCounter";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import AppUpdateCard from "@/components/AppUpdateCard.vue";
import {
  animationsEnabled,
  setAnimationsEnabled,
  calculatorTotalCurrency,
  setCalculatorTotalCurrency,
  quickAccessRows,
  setQuickAccessRows,
  QUICK_ACCESS_MAX_ROWS,
  quickAccessShowCode,
  setQuickAccessShowCode,
  quickAccessShowName,
  setQuickAccessShowName,
  quickAccessShowRemove,
  setQuickAccessShowRemove,
  QUICK_ACCESS_COLORS,
  QUICK_ACCESS_TEXT_COLORS,
  quickAccessFontScale,
  setQuickAccessFontScale,
  QUICK_ACCESS_FONT_MIN,
  QUICK_ACCESS_FONT_MAX,
  quickAccessCardStyle,
} from "@/lib/display";
import { useTheme } from "vuetify";

const auth = useAuthStore();
const { setClientRequired: setClientRequiredState } = useClient();
const { setSalespersonPerInvoiceRequired: setSalespersonPerInvoiceRequiredState } = useSalesperson();
const { refresh: refreshVisitorCounter } = useVisitorCounter();
const theme = useTheme();
const tab = ref("general");

const settingsTabs = [
  { value: "general", label: "General", icon: "mdi-cog-outline" },
  { value: "printing", label: "Printing", icon: "mdi-printer-outline" },
  { value: "attendance", label: "Attendance", icon: "mdi-clipboard-check-outline" },
  { value: "money", label: "Money", icon: "mdi-cash-multiple" },
  { value: "coupons", label: "Coupons", icon: "mdi-ticket-percent-outline" },
  { value: "behavior", label: "Behavior", icon: "mdi-button-pointer" },
  { value: "admin", label: "Security", icon: "mdi-shield-account-outline" },
] as const;

// Static index of every settings "panel" (its tab + searchable text) used by
// the search box below. Keep in sync with the v-show="panelMatches(...)" on
// each panel in the template.
// Backed by the resync store (src/stores/resync.ts), not local refs: a
// sync keeps running on the Go side regardless of what page is open, and a
// requiresConfirmation response must not get lost if the operator
// navigates away from Settings before it lands. The store persists for the
// app's lifetime; this page just reads/drives it.
const resync = useResyncStore();
resync.attachProgressListener();
const resyncRunning = computed(() => resync.running);
const resyncApplyStock = computed({
  get: () => resync.applyStock,
  set: (v: boolean) => { resync.applyStock = v; },
});
const resyncProgress = computed(() => resync.progress);
const resyncError = computed(() => resync.error);
// The raw error is the full Go wrap chain (useful for support/logs); the
// operator-facing alert leads with a plain-language reason when one is
// recognized (see friendlySyncErrorReason) and only falls back to showing
// that raw chain when it isn't.
const resyncErrorFriendly = computed(() => resyncError.value ? friendlySyncErrorReason(resyncError.value) : null);
const resyncReport = computed(() => resync.report);
const pendingDeactivation = computed(() => resync.pendingDeactivation);
const showDeactivationConfirm = computed(() => resync.showDeactivationConfirm);
const deactivationConfirming = computed(() => resync.deactivationConfirming);

function runResync(dryRun: boolean, confirmDeactivation = false) {
  return resync.run(dryRun, confirmDeactivation);
}
function confirmDeactivationAndResync() {
  return resync.confirmDeactivationAndRun();
}
function cancelDeactivation() {
  resync.cancelDeactivation();
}

const panelMeta: Record<string, { tab: string; title: string; keywords: string }> = {
  "general-quick-access-colors": { tab: "general", title: "Quick access colors", keywords: "quick access card color colour background text font source kashi central managed till" },
  "general-appearance": { tab: "general", title: "Appearance", keywords: "dark theme animations mode display quick access rows panel hide code name card remove x button text font size bigger smaller" },
  "general-branch": { tab: "general", title: "Branch", keywords: "branch name code sales inventory" },
  "general-database": { tab: "general", title: "Database", keywords: "database path choose switch" },
  "general-branches": { tab: "general", title: "Branches", keywords: "branches switch default active add branch multiple instances" },
  "general-sync": { tab: "general", title: "Central sync (kashi)", keywords: "central sync kashi server url branch code api key upload before close shift wait pending offline" },
  "general-targets": { tab: "general", title: "Targets", keywords: "targets revenue goal progress date range" },
  "general-admin-legacy": { tab: "general", title: "kashi-admin (legacy)", keywords: "remote admin api listen address token legacy tui" },
  "attendance-machine": { tab: "attendance", title: "Attendance machine", keywords: "akuvox ip username password manager duplicate interval enable cashier history auto close stale shifts" },
  "printing-printer": { tab: "printing", title: "Printer & customer screen", keywords: "receipt printer size width height print customer screen com port qr code close shift photo upload" },
  "printing-receipt": { tab: "printing", title: "Receipt settings", keywords: "market name phone description branch phone register return policy social platforms instagram facebook website font body size title size cut paper sample" },
  "money-general": { tab: "money", title: "Money settings", keywords: "vat tax rate exchange rate overpayment rounding mode currency calculator total currency auto append lbp zeros 000 confirm change overchange refund more than owed" },
  "money-float": { tab: "money", title: "Opening cash float", keywords: "opening cash float usd lbp drawer shift close count reconciliation expected enable disable feature toggle balance" },
  "money-exchange": { tab: "money", title: "Exchange settings", keywords: "exchange window days hours return" },
  "money-currencies": { tab: "money", title: "Currencies & rates", keywords: "currency code name symbol units per usd cash rounding unit active add currency" },
  "money-payment-methods": { tab: "money", title: "Payment methods", keywords: "payment method cashbox account currency" },
  "money-price-lists": { tab: "money", title: "Price lists", keywords: "price list override product code black friday active" },
  "money-discount-lists": { tab: "money", title: "Discount lists", keywords: "discount list override product code percent active" },
  "coupons-types": { tab: "coupons", title: "Coupon types", keywords: "coupon type value currency reference only active" },
  "coupons-issued": { tab: "coupons", title: "Issued coupons", keywords: "coupon issue code new" },
  "behavior-custom-pricing": { tab: "behavior", title: "Custom item pricing", keywords: "custom item price discount per unit editing" },
  "behavior-price-change-mode": { tab: "behavior", title: "Price change mode", keywords: "price change manual override discount lever" },
  "behavior-invoice-keyboard": { tab: "behavior", title: "Invoice keyboard mode", keywords: "invoice keyboard qty discount price number pad" },
  "behavior-product-search": { tab: "behavior", title: "Product search button", keywords: "product search magnifier sale screen find lookup disable hide" },
  "behavior-scan-by-code": { tab: "behavior", title: "Product code in scan field", keywords: "barcode scan code sku variant type enter" },
  "behavior-always-anonymous": { tab: "behavior", title: "Always Anonymous client", keywords: "always anonymous client toggle button prefer not to say" },
  "behavior-salesperson-per-invoice": { tab: "behavior", title: "Salesperson per invoice", keywords: "salesperson per invoice require force clear carry forward previous cashier pick" },
  "behavior-non-admin-month-nav": { tab: "behavior", title: "Income chart month navigation", keywords: "monthly income chart non-admin cashier back forward prev next month browse lock admin only" },
  "behavior-visitor-counter": { tab: "behavior", title: "People visits counter", keywords: "people visits counter footfall traffic side menu nav drawer up down orange chip monthly income" },
  "admin-app-updates": { tab: "admin", title: "App updates", keywords: "update upgrade version install roll back rollback previous release download" },
  "admin-page-access": { tab: "admin", title: "Page access", keywords: "page access lock unlock non-admin clients inventory transfers attendance salespersons permissions" },
  "admin-change-password": { tab: "admin", title: "Change admin password", keywords: "change admin password current new confirm account security login sign in" },
  "admin-cashier-pin": { tab: "admin", title: "Cashier login PIN", keywords: "cashier pin code login keypad 4 digit change shared" },
  "admin-users": { tab: "admin", title: "Users", keywords: "users username role status add user edit delete akuvox userid" },
  "admin-salespersons": { tab: "admin", title: "Salespersons", keywords: "salespersons full name attendance id add edit delete" },
};

const settingsSearch = ref("");
const normalizedSearch = computed(() => settingsSearch.value.trim().toLowerCase());

function panelMatches(key: string): boolean {
  if (!normalizedSearch.value) return true;
  const meta = panelMeta[key];
  if (!meta) return true;
  return `${meta.title} ${meta.keywords}`.toLowerCase().includes(normalizedSearch.value);
}

const tabsWithMatches = computed(() => {
  if (!normalizedSearch.value) return null;
  const set = new Set<string>();
  for (const key of Object.keys(panelMeta)) {
    if (panelMatches(key)) set.add(panelMeta[key]!.tab);
  }
  return set;
});

watch(normalizedSearch, () => {
  const matches = tabsWithMatches.value;
  if (!matches || matches.has(tab.value) || !matches.size) return;
  tab.value = matches.values().next().value as string;
});
const databasePath = ref("");
const databaseSwitching = ref(false);

const branchOptions = ref<{ code: string; title: string }[]>([]);
const selectedBranchCode = ref("");
const activeBranchCode = ref("");
const defaultBranchCode = ref("");
const branchSwitching = ref(false);
const creatingBranch = ref(false);
const settingDefaultBranch = ref(false);
const branchActionError = ref("");
const newBranch = reactive({ name: "", code: "", apiKey: "", centralApiUrl: "" });
const importingHistory = ref(false);
const historyImportResult = ref<Record<string, number> | null>(null);
const historyImportError = ref("");
const darkThemeEnabled = computed(() => theme.name.value === "dark");
const themeSaving = ref(false);

async function setDarkTheme(value: boolean | null) {
  const previous = theme.name.value;
  const next = value ? "dark" : "light";
  if (next === previous || themeSaving.value) return;

  theme.change(next);
  themeSaving.value = true;
  try {
    await SetTheme(next);
    notify(`${value ? "Dark" : "Light"} theme enabled`);
  } catch (error) {
    theme.change(previous);
    notify(error instanceof Error ? error.message : "Failed to save theme", "red");
  } finally {
    themeSaving.value = false;
  }
}

const form = reactive({
  branchName: "",
  branchCode: "CN",
  apiKey: "",
  centralApiUrl: "",
  printerID: "",
  printerSize: "80mm",
  receiptPrinter: "",
  screenPort: "",
  receiptWidth: 80,
  receiptHeight: 297,
  taxRate: 0,
  roundingMode: "half_up",
  roundingCurrency: "LBP",
  // First half of the split total; the rounding currency is the second.
  splitPrimaryCurrency: "USD",
  splitPrimaryFirst: true,
  // 0 = fall back to the primary currency's own cash_rounding_unit.
  splitPrimaryStep: 0,
  receiptEnabled: false,
  autoCloseStaleShiftsEnabled: false,
  salesInventoryId: 0,
  exchangeRate: 90000,
  maxOverpayment: 1,
  allowOverpayment: false,
  defaultOpeningFloatUsd: 0,
  defaultOpeningFloatLbp: 0,
  exchangeWindowDays: 3,
  exchangeWindowHours: 0,
  marketName: "",
  marketPhone: "",
  branchPhone: "",
  registerName: "POS-01",
  marketDescription: "Shop for all",
  returnPolicy: "Exchanges accepted within 3 days. Items must be unworn, unused, unwashed, undamaged, with tags. Present this receipt.",
  website: "yourwebsite.com",
  instagram: "@your_instagram",
  socialPlatforms: [] as string[],
  socialHandles: {} as Record<string, string>,
  receiptFont: "Belleza",
  receiptTitleSize: 21,
  receiptBodySize: 11,
  receiptBodyFont: "Printer Font A",
  receiptCutoff: true,
  customItemDiscountsEnabled: false,
  customItemPricesEnabled: false,
  perUnitItemPricesEnabled: false,
  priceChangeManualOverrideEnabled: false,
  invoiceKeyboardModeEnabled: false,
  productSearchEnabled: true,
  scanByCodeEnabled: true,
  visitorCounterEnabled: false,
  clientRequired: true,
  salespersonPerInvoiceRequired: false,
  nonAdminMonthNavEnabled: false,
  hidePaymentMethodCurrency: false,
  autoAppendLbpZeros: false,
  overchangeConfirmationEnabled: true,
  closeShiftRequiresSync: false,
  openingFloatEnabled: true,
  countOpeningCashOnClose: true,
  shiftReceiptUploadMode: 2,
  adminEnabled: false,
  adminListen: "127.0.0.1:8787",
  adminToken: "",
  akuvoxIP: "",
  akuvoxUsername: "",
  akuvoxPassword: "",
  attendanceEnabled: false,
  attendanceApproverPassword: "",
  attendanceDuplicateIntervalSeconds: 30,
  attendanceCashierHistory: false,
});
const roundingModes = [
  { title: "Half up (standard)", value: "half_up" },
  { title: "Always down", value: "down" },
  { title: "Always up", value: "up" },
];
const currencies = ref<any[]>([]);
const roundingCurrencies = computed(() =>
  currencies.value
    .filter((c) => c.isActive)
    .map((c) => ({
      title: `${c.name} (${c.cashRoundingUnit > 1 ? `nearest ${c.cashRoundingUnit.toLocaleString()} ${c.code}` : `nearest ${c.symbol}1`})`,
      value: c.code,
    })),
);
// The default currency (e.g. USC, the internal accounting cent) is a
// bookkeeping unit, not a currency a cashier should ever see as a total.
const calculatorDisplayCurrencies = computed(() =>
  currencies.value
    .filter((c) => c.isActive && !c.isDefault)
    .map((c) => ({ title: c.name, value: c.code })),
);
const paymentMethods = ref<any[]>([]);
// "On Account"/"On Coupon" are internal bookkeeping placeholders, not
// selectable tenders - hide them from the admin-facing list/reorder UI.
const orderablePaymentMethods = computed(() =>
  paymentMethods.value.filter((m) => !isSpecialPaymentMethod(m.name)),
);
const editingCurrencyCode = ref<string | null>(null);
const currencySaving = ref(false);
const autoAppendLbpZerosSaving = ref(false);
const overchangeConfirmationSaving = ref(false);
const closeShiftRequiresSyncSaving = ref(false);
const openingFloatEnabledSaving = ref(false);
const shiftReceiptUploadModeSaving = ref(false);
const shiftReceiptUploadModes = [
  { title: "Hybrid (office when online, else this machine)", value: 2 },
  { title: "Local only (this machine, over shop Wi-Fi)", value: 3 },
  { title: "Central only (straight to the office)", value: 1 },
];
const countOpeningCashOnCloseSaving = ref(false);
const editingPaymentMethodId = ref<number | null>(null);
const paymentMethodSaving = ref(false);

async function loadCurrencies() {
  try {
    currencies.value = (await ListCurrencies()) as any[];
  } catch {
    currencies.value = [];
  }
}

async function loadPaymentMethodsList() {
  try {
    paymentMethods.value = (await ListPaymentMethods()) as any[];
  } catch {
    paymentMethods.value = [];
  }
}

const priceLists = ref<any[]>([]);

const discountLists = ref<any[]>([]);

const manageListDialog = ref(false);
const manageListType = ref<"price" | "discount">("price");
const manageListId = ref<number | null>(null);
const manageListName = ref("");
const manageListItems = ref<any[]>([]);

async function loadPriceLists() {
  try {
    priceLists.value = (await ListPriceLists()) as any[];
  } catch {
    priceLists.value = [];
  }
}

async function loadDiscountLists() {
  try {
    discountLists.value = (await ListDiscountLists()) as any[];
  } catch {
    discountLists.value = [];
  }
}

async function loadManageListItems() {
  if (!manageListId.value) return;
  try {
    manageListItems.value =
      manageListType.value === "price"
        ? ((await ListPriceListItems(manageListId.value)) as any[])
        : ((await ListDiscountListItems(manageListId.value)) as any[]);
  } catch {
    manageListItems.value = [];
  }
}

async function openManageList(type: "price" | "discount", list: any) {
  manageListType.value = type;
  manageListId.value = list.id;
  manageListName.value = list.name;
  manageListDialog.value = true;
  await loadManageListItems();
}

const inventories = ref<any[]>([]);
const socialPlatformOptions = [
  { title: "", value: "facebook", icon: "mdi-facebook", label: "Facebook" },
  { title: "", value: "tiktok", icon: "mdi-music-note", label: "TikTok" },
  { title: "", value: "instagram", icon: "mdi-instagram", label: "Instagram" },
  { title: "", value: "threads", icon: "mdi-at", label: "Threads" },
];

function platformIcon(platform: string) {
  return socialPlatformOptions.find((item) => item.value === platform)?.icon ?? "mdi-at";
}

function platformLabel(platform: string) {
  return socialPlatformOptions.find((item) => item.value === platform)?.label ?? platform;
}
const serialPorts = ref<string[]>([]);
const printers = ref<string[]>([]);
// Read-only mirror of what kashi's admin has set for this branch (see
// sync_pull.go) -- there is no create/edit here, targets are managed
// centrally only.
const branchTargets = ref<
  { id: number; dateFrom: string; dateTo: string; targetAmountCents: number; achievedCents: number; color: string }[]
>([]);
const saving = ref(false);
const couponTypes = ref<any[]>([]);
const coupons = ref<any[]>([]);
const couponTypeForm = reactive({ name: "", value: 50, currency: "USD", referenceOnly: false, isActive: true, calculateOnCheckout: false, codeRequired: true });
const editingCouponTypeId = ref<number | null>(null);
const couponTypeSaving = ref(false);
const couponCodeForm = ref("");
const couponIssueTypeId = ref<number | null>(null);
const couponIssuing = ref(false);
const pricingSettingsSaving = ref(false);
const { pageUnlocks } = usePageUnlocks();
const pageUnlockSaving = ref(false);
const pageUnlockLabels: Record<string, string> = {
  clients: "Clients",
  inventory: "Inventory",
  products: "Products",
  transfers: "Transfers",
  attendance: "Attendance",
  salespersons: "Salespersons",
};
const adminSettingsSaving = ref(false);
const samplePrinting = ref(false);
const showApiKey = ref(false);
const showAdminToken = ref(false);
const settingsLoading = ref(true);
const settingsLoadError = ref(false);
const passwordForm = reactive({ current: "", newPass: "", confirm: "" });
const showPw = ref(false);
const changingPassword = ref(false);
const cashierPinForm = reactive({ newPin: "", confirm: "" });
const cashierPinCurrent = ref("");
const showCashierPin = ref(false);
const savingCashierPin = ref(false);

const users = ref<any[]>([]);
const roles = ref<any[]>([]);
const userHeaders = [
  { title: "Username", key: "username" },
  { title: "Role", key: "role" },
  { title: "Status", key: "active" },
  { title: "", key: "actions", sortable: false, align: "end" as const },
];

const userDialog = ref(false);
const editingUser = ref<any>(null);
const editingSyncedUser = computed(() => !!editingUser.value?.isSynced);
const userForm = reactive({ username: "", password: "", roleId: 1, attendanceUserId: "" });
const showUpw = ref(false);
const savingUser = ref(false);

const deleteDialog = ref(false);
const deleteTarget = ref<any>(null);

const salespersons = ref<any[]>([]);
const salespersonHeaders = [
  { title: "", key: "color", sortable: false, width: "40px" },
  { title: "Name", key: "fullName" },
  { title: "Attendance ID", key: "attendanceUserId" },
  { title: "Status", key: "isActive", sortable: false },
  { title: "", key: "actions", sortable: false, align: "end" as const },
];
// When central sync (kashi) is configured this branch's salesperson roster is
// owned upstream and only the local attendance ID is editable. With no central
// server set, salespersons are managed entirely locally (add/rename/delete).
const salespersonsSyncManaged = computed(() => !!form.centralApiUrl?.trim());
// Mirrors the Go guards in salespersons.go: names are editable only in
// standalone mode.
const salespersonNameEditable = computed(() => !salespersonsSyncManaged.value && !editingSalesperson.value?.isSynced);
const salespersonDialog = ref(false);
const editingSalesperson = ref<any>(null);
const salespersonName = ref("");
const salespersonAttendanceUserId = ref("");
const savingSalesperson = ref(false);
const deleteSalespersonDialog = ref(false);
const deleteSalespersonTarget = ref<any>(null);

const snackbar = reactive({ show: false, text: "", color: "success" });

const adminBridge = () => (window as any).go?.main?.App;

type QuickAccessColorItem = {
  id: number;
  code: string;
  name: string;
  price: number;
  quickAccessColor: string;
  quickAccessTextColor: string;
};
const quickAccessItems = ref<QuickAccessColorItem[]>([]);

async function loadQuickAccessItems() {
  try {
    const source = await GetQuickAccessSource();
    quickAccessMode.value = source.mode;
    quickAccessKashiAvailable.value = source.kashiAvailable;
    quickAccessItems.value = ((await ListQuickAccessProducts()) ?? []) as QuickAccessColorItem[];
  } catch {}
}

// Where this register's Quick Access cards come from: its own list ("till")
// or its branch's list in kashi ("kashi"). Flipping here reaches kashi
// through the sync outbox; kashi can flip it too, and the later flip wins.
const quickAccessMode = ref("till");
const quickAccessKashiAvailable = ref(false);
const changingQuickAccessMode = ref(false);

async function changeQuickAccessMode(mode: string) {
  if (!mode || mode === quickAccessMode.value) return;
  changingQuickAccessMode.value = true;
  try {
    await SetQuickAccessMode(mode);
    await loadQuickAccessItems();
  } catch (error) {
    notify(`Could not change where Quick Access comes from: ${error}`, "error");
  } finally {
    changingQuickAccessMode.value = false;
  }
}

async function setQuickAccessItemColor(
  item: QuickAccessColorItem,
  change: Partial<Pick<QuickAccessColorItem, "quickAccessColor" | "quickAccessTextColor">>,
) {
  const previous = { quickAccessColor: item.quickAccessColor, quickAccessTextColor: item.quickAccessTextColor };
  Object.assign(item, change);
  try {
    await SetQuickAccessColor(item.id, item.quickAccessColor ?? "", item.quickAccessTextColor ?? "");
  } catch (error) {
    Object.assign(item, previous);
    notify(`Could not save color: ${error}`, "error");
  }
}

const exportingQuickAccess = ref(false);

async function exportQuickAccessCsv() {
  exportingQuickAccess.value = true;
  try {
    const path = await ExportQuickAccessCSV();
    if (path) notify(`Quick access exported to ${path}`);
  } catch (e: any) {
    notify(e?.toString() || "Could not export quick access", "error");
  } finally {
    exportingQuickAccess.value = false;
  }
}

function notify(text: string, color = "success") {
  snackbar.text = text;
  snackbar.color = color;
  snackbar.show = true;
}

async function loadCouponTypes() {
  try { couponTypes.value = (await ListCouponTypes()) as any[]; } catch { couponTypes.value = []; }
  try { coupons.value = (await ListCoupons()) as any[]; } catch { coupons.value = []; }
}

async function removeCoupon(coupon: any) {
  if (!window.confirm(`Delete coupon ${coupon.code}?`)) return;
  try { await DeleteCoupon(coupon.code); await loadCouponTypes(); notify("Coupon deleted"); }
  catch (error) { notify(error instanceof Error ? error.message : "Failed to delete coupon", "red"); }
}

async function removeCouponType(couponType: any) {
  if (!window.confirm(`Delete coupon type ${couponType.name}?`)) return;
  try { await DeleteCouponType(Number(couponType.id)); await loadCouponTypes(); notify("Coupon type deleted"); }
  catch (error) { notify(error instanceof Error ? error.message : "Failed to delete coupon type", "red"); }
}

function editCouponType(couponType: any) {
  editingCouponTypeId.value = Number(couponType.id);
  couponTypeForm.name = couponType.name;
  couponTypeForm.value = Number(couponType.valueCents) / 100;
  couponTypeForm.currency = couponType.currencyCode || "USD";
  couponTypeForm.referenceOnly = Boolean(couponType.referenceOnly);
  couponTypeForm.isActive = Number(couponType.isActive) !== 0;
  couponTypeForm.calculateOnCheckout = Boolean(couponType.calculateOnCheckout);
  couponTypeForm.codeRequired = couponType.codeRequired !== false;
}

function cancelCouponTypeEdit() {
  editingCouponTypeId.value = null;
  couponTypeForm.name = "";
  couponTypeForm.value = 50;
  couponTypeForm.currency = "USD";
  couponTypeForm.referenceOnly = false;
  couponTypeForm.isActive = true;
  couponTypeForm.calculateOnCheckout = false;
  couponTypeForm.codeRequired = true;
}

async function saveCouponType() {
  if (!couponTypeForm.name.trim() || Number(couponTypeForm.value) <= 0) return;
  couponTypeSaving.value = true;
  try {
    const valueCents = Math.round(Number(couponTypeForm.value) * 100);
    const referenceOnly = couponTypeForm.codeRequired && couponTypeForm.referenceOnly;
    if (editingCouponTypeId.value) {
      await UpdateCouponType(editingCouponTypeId.value, couponTypeForm.name.trim(), couponTypeForm.currency, valueCents, couponTypeForm.isActive, referenceOnly, couponTypeForm.calculateOnCheckout, couponTypeForm.codeRequired);
    } else {
      await CreateCouponType(couponTypeForm.name.trim(), couponTypeForm.currency, valueCents, referenceOnly, couponTypeForm.calculateOnCheckout, couponTypeForm.codeRequired);
    }
    const wasEditing = Boolean(editingCouponTypeId.value);
    cancelCouponTypeEdit();
    await loadCouponTypes();
    notify(wasEditing ? "Coupon type updated" : "Coupon type added");
  } catch (error) { notify(error instanceof Error ? error.message : "Failed to add coupon type", "red"); }
  finally { couponTypeSaving.value = false; }
}

async function issueCoupon() {
  if (!couponCodeForm.value.trim() || !couponIssueTypeId.value) return;
  couponIssuing.value = true;
  try {
    await CreateCoupon(couponCodeForm.value.trim(), couponIssueTypeId.value, 0);
    couponCodeForm.value = "";
    notify("Coupon issued");
  } catch (error) { notify(error instanceof Error ? error.message : "Failed to issue coupon", "red"); }
  finally { couponIssuing.value = false; }
}

async function loadSettings() {
  settingsLoading.value = true;
  settingsLoadError.value = false;
  try {
    const s = await GetAppSettings();
    if (s)
      Object.assign(form, {
        branchName: s.branchName ?? "",
        branchCode: s.branchCode ?? "CN",
        apiKey: s.apiKey ?? "",
        centralApiUrl: s.centralApiUrl ?? "",
        printerID: s.printerID ?? "",
        printerSize: s.printerSize ?? "80mm",
        receiptPrinter: s.receiptPrinter ?? "",
        screenPort: s.screenPort ?? "",
        receiptWidth: Number(s.receiptWidth ?? 80),
        receiptHeight: Number(s.receiptHeight ?? 297),
        taxRate: Number(s.taxRate ?? 0),
        roundingMode: s.roundingMode ?? "half_up",
        roundingCurrency: s.roundingCurrency ?? "LBP",
        splitPrimaryCurrency: s.splitPrimaryCurrency || "USD",
        splitPrimaryFirst: s.splitPrimaryFirst !== false,
        splitPrimaryStep: Number(s.splitPrimaryStep) || 0,
        receiptEnabled: Boolean(s.receiptEnabled),
        autoCloseStaleShiftsEnabled: Boolean(s.autoCloseStaleShiftsEnabled),
        salesInventoryId: Number(s.salesInventoryId ?? 0),
        exchangeRate: Number(s.exchangeRate ?? 90000),
        maxOverpayment: Number(s.maxOverpaymentCents ?? 0) / 100,
        allowOverpayment: Number(s.maxOverpaymentCents ?? 0) > 0,
        defaultOpeningFloatUsd: Number(s.defaultOpeningFloatUsd ?? 0) / 100,
        defaultOpeningFloatLbp: Number(s.defaultOpeningFloatLbp ?? 0),
        exchangeWindowDays: Math.floor(Number(s.exchangeWindowHours ?? 72) / 24),
        exchangeWindowHours: Number(s.exchangeWindowHours ?? 72) % 24,
        marketName: s.marketName ?? "",
        marketPhone: s.marketPhone ?? "",
        branchPhone: s.branchPhone ?? "",
        registerName: s.registerName ?? s.counterNumber ?? "POS-01",
        marketDescription: s.marketDescription ?? "Shop for all",
        returnPolicy: s.returnPolicy ?? "",
        website: s.website ?? "yourwebsite.com",
        instagram: s.instagram ?? "@your_instagram",
        socialPlatforms: Array.isArray(s.socialPlatforms) ? s.socialPlatforms : [],
        socialHandles: typeof s.socialHandles === "object" && s.socialHandles ? s.socialHandles : {},
        receiptFont: s.receiptFont ?? "Belleza",
        receiptTitleSize: Number(s.receiptTitleSize ?? 21),
        receiptBodySize: Number(s.receiptBodySize ?? 11),
        receiptBodyFont: s.receiptBodyFont ?? "Printer Font A",
        receiptCutoff: s.receiptCutoff !== false,
        customItemDiscountsEnabled: Boolean(s.customItemDiscountsEnabled),
        customItemPricesEnabled: Boolean(s.customItemPricesEnabled),
        perUnitItemPricesEnabled: Boolean(s.perUnitItemPricesEnabled),
        priceChangeManualOverrideEnabled: Boolean(s.priceChangeManualOverrideEnabled),
        invoiceKeyboardModeEnabled: Boolean(s.invoiceKeyboardModeEnabled),
        productSearchEnabled: s.productSearchEnabled !== false,
        scanByCodeEnabled: s.scanByCodeEnabled !== false,
        visitorCounterEnabled: Boolean(s.visitorCounterEnabled),
        clientRequired: s.clientRequired !== false,
        salespersonPerInvoiceRequired: Boolean(s.salespersonPerInvoiceRequired),
        nonAdminMonthNavEnabled: Boolean(s.nonAdminMonthNavEnabled),
        hidePaymentMethodCurrency: Boolean(s.hidePaymentMethodCurrency),
        autoAppendLbpZeros: Boolean(s.autoAppendLbpZeros),
        overchangeConfirmationEnabled: s.overchangeConfirmationEnabled !== false,
        closeShiftRequiresSync: Boolean(s.closeShiftRequiresSync),
        openingFloatEnabled: s.openingFloatEnabled !== false,
        countOpeningCashOnClose: s.countOpeningCashOnClose !== false,
        shiftReceiptUploadMode: Number(s.shiftReceiptUploadMode ?? 2),
        akuvoxIP: s.attendance?.ip ?? "",
        akuvoxUsername: s.attendance?.username ?? "",
        attendanceEnabled: Boolean(s.attendance?.enabled),
        attendanceApproverPassword: "",
        attendanceDuplicateIntervalSeconds: Number(s.attendance?.duplicateIntervalSeconds ?? 30),
        attendanceCashierHistory: Boolean(s.attendance?.cashierHistory),
      });
    const admin = await adminBridge()?.GetAdminConfig();
    Object.assign(form, {
      adminEnabled: Boolean(admin?.enabled),
      adminListen: admin?.listen ?? "127.0.0.1:8787",
      adminToken: admin?.token ?? "",
    });
  } catch {
    settingsLoadError.value = true;
  } finally {
    settingsLoading.value = false;
  }
}

async function loadDatabaseStatus() {
  try {
    const status = await GetDatabaseStatus();
    databasePath.value = String(status?.path ?? "");
  } catch {}
}

async function chooseDatabase() {
  databaseSwitching.value = true;
  try {
    const path = await ChooseDatabase();
    if (path) WindowReloadApp();
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to load database", "red");
  } finally {
    databaseSwitching.value = false;
  }
}

async function loadBranches() {
  try {
    const result = await ListBranches();
    const branches = Array.isArray(result?.branches) ? result.branches : [];
    branchOptions.value = branches.map((b: { code: string; name: string }) => ({
      code: b.code,
      title: `${b.name} (${b.code})`,
    }));
    activeBranchCode.value = String(result?.active ?? "");
    defaultBranchCode.value = String(result?.default ?? "");
    selectedBranchCode.value = activeBranchCode.value;
  } catch {}
}

async function switchBranch(code: string) {
  if (!code || code === activeBranchCode.value) return;
  branchSwitching.value = true;
  branchActionError.value = "";
  try {
    await SwitchBranch(code);
    WindowReloadApp();
  } catch (error) {
    branchActionError.value = error instanceof Error ? error.message : String(error);
    selectedBranchCode.value = activeBranchCode.value;
  } finally {
    branchSwitching.value = false;
  }
}

async function setDefaultBranch(code: string) {
  if (!code) return;
  settingDefaultBranch.value = true;
  branchActionError.value = "";
  try {
    await SetDefaultBranch(code);
    defaultBranchCode.value = code;
    notify("Default branch updated");
  } catch (error) {
    branchActionError.value = error instanceof Error ? error.message : String(error);
  } finally {
    settingDefaultBranch.value = false;
  }
}

async function createBranch() {
  creatingBranch.value = true;
  branchActionError.value = "";
  try {
    await CreateBranch(newBranch.name, newBranch.code, newBranch.apiKey, newBranch.centralApiUrl);
    WindowReloadApp();
  } catch (error) {
    branchActionError.value = error instanceof Error ? error.message : String(error);
  } finally {
    creatingBranch.value = false;
  }
}

async function importBranchHistory() {
  importingHistory.value = true;
  historyImportError.value = "";
  historyImportResult.value = null;
  try {
    const result = await PullBranchHistory();
    historyImportResult.value = result as Record<string, number>;
  } catch (error) {
    historyImportError.value = error instanceof Error ? error.message : String(error);
  } finally {
    importingHistory.value = false;
  }
}

async function saveAdminSettings() {
  adminSettingsSaving.value = true;
  try {
    await adminBridge()?.UpdateAdminConfig(Boolean(form.adminEnabled), form.adminListen.trim(), form.adminToken);
    notify(form.adminEnabled ? "Remote admin settings saved" : "Remote admin disabled");
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to save remote admin settings", "red");
  } finally {
    adminSettingsSaving.value = false;
  }
}

async function saveGeneralSettings() {
  await saveSettings();
  await saveAdminSettings();
}

async function setCustomDiscountsEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.customItemDiscountsEnabled;
  form.customItemDiscountsEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetCustomItemDiscountsEnabled(enabled);
    notify(`Custom product discounts ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.customItemDiscountsEnabled = previous;
    notify("Failed to update custom product discounts", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setCustomPricesEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.customItemPricesEnabled;
  form.customItemPricesEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetCustomItemPricesEnabled(enabled);
    notify(`Custom item prices ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.customItemPricesEnabled = previous;
    notify("Failed to update custom item prices", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setPerUnitPricesEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.perUnitItemPricesEnabled;
  form.perUnitItemPricesEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetPerUnitItemPricesEnabled(enabled);
    notify(`Per-unit price and discount editing ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.perUnitItemPricesEnabled = previous;
    notify("Failed to update per-unit price and discount editing", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setPriceChangeManualOverrideEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.priceChangeManualOverrideEnabled;
  form.priceChangeManualOverrideEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetPriceChangeManualOverrideEnabled(enabled);
    notify(`Price changes now use ${enabled ? "manual override" : "discount"} mode`);
  } catch {
    form.priceChangeManualOverrideEnabled = previous;
    notify("Failed to update price change mode", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setInvoiceKeyboardModeEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.invoiceKeyboardModeEnabled;
  form.invoiceKeyboardModeEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetInvoiceKeyboardModeEnabled(enabled);
    notify(`Invoice keyboard mode ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.invoiceKeyboardModeEnabled = previous;
    notify("Failed to update invoice keyboard mode", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setProductSearchEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.productSearchEnabled;
  form.productSearchEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetProductSearchEnabled(enabled);
    notify(`Product search button ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.productSearchEnabled = previous;
    notify("Failed to update product search button", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setScanByCodeEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.scanByCodeEnabled;
  form.scanByCodeEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetScanByCodeEnabled(enabled);
    notify(`Product codes in the scan field ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.scanByCodeEnabled = previous;
    notify("Failed to update the scan field setting", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setVisitorCounterEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.visitorCounterEnabled;
  form.visitorCounterEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetVisitorCounterEnabled(enabled);
    await refreshVisitorCounter();
    notify(`People visits counter ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.visitorCounterEnabled = previous;
    notify("Failed to update people visits counter", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setClientRequired(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.clientRequired;
  form.clientRequired = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetClientRequired(enabled);
    setClientRequiredState(enabled);
    notify(`Client on checkout is now ${enabled ? "required" : "optional"}`);
  } catch {
    form.clientRequired = previous;
    notify("Failed to update client requirement", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setSalespersonPerInvoiceRequired(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.salespersonPerInvoiceRequired;
  form.salespersonPerInvoiceRequired = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetSalespersonPerInvoiceRequired(enabled);
    setSalespersonPerInvoiceRequiredState(enabled);
    notify(
      enabled
        ? "Salesperson must now be picked on every invoice"
        : "Salesperson now carries forward between invoices",
    );
  } catch {
    form.salespersonPerInvoiceRequired = previous;
    notify("Failed to update salesperson requirement", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setNonAdminMonthNavEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.nonAdminMonthNavEnabled;
  form.nonAdminMonthNavEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetNonAdminMonthNavEnabled(enabled);
    notify(
      enabled
        ? "Non-admins can now browse other months on the income chart"
        : "Income chart month navigation is now admin-only",
    );
  } catch {
    form.nonAdminMonthNavEnabled = previous;
    notify("Failed to update income chart month navigation setting", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setHidePaymentMethodCurrency(value: boolean | null) {
  const hide = Boolean(value);
  const previous = form.hidePaymentMethodCurrency;
  form.hidePaymentMethodCurrency = hide;
  paymentMethodSaving.value = true;
  try {
    await SetHidePaymentMethodCurrency(hide);
    notify(`Payment method currency ${hide ? "hidden" : "shown"}`);
  } catch {
    form.hidePaymentMethodCurrency = previous;
    notify("Failed to update payment method currency visibility", "red");
  } finally {
    paymentMethodSaving.value = false;
  }
}

async function setAutoAppendLbpZeros(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.autoAppendLbpZeros;
  form.autoAppendLbpZeros = enabled;
  autoAppendLbpZerosSaving.value = true;
  try {
    await SetAutoAppendLbpZerosEnabled(enabled);
    notify(`Auto-append LBP zeros ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.autoAppendLbpZeros = previous;
    notify("Failed to update auto-append LBP zeros", "red");
  } finally {
    autoAppendLbpZerosSaving.value = false;
  }
}

async function setOverchangeConfirmationEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.overchangeConfirmationEnabled;
  form.overchangeConfirmationEnabled = enabled;
  overchangeConfirmationSaving.value = true;
  try {
    await SetOverchangeConfirmationEnabled(enabled);
    notify(`Over-change confirmation ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.overchangeConfirmationEnabled = previous;
    notify("Failed to update over-change confirmation", "red");
  } finally {
    overchangeConfirmationSaving.value = false;
  }
}

async function setCloseShiftRequiresSync(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.closeShiftRequiresSync;
  form.closeShiftRequiresSync = enabled;
  closeShiftRequiresSyncSaving.value = true;
  try {
    await SetCloseShiftRequiresSyncEnabled(enabled);
    notify(`Upload before closing a shift ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.closeShiftRequiresSync = previous;
    notify("Failed to update upload-before-close setting", "red");
  } finally {
    closeShiftRequiresSyncSaving.value = false;
  }
}

async function setShiftReceiptUploadMode(value: number | null) {
  const mode = Number(value ?? 2);
  const previous = form.shiftReceiptUploadMode;
  form.shiftReceiptUploadMode = mode;
  shiftReceiptUploadModeSaving.value = true;
  try {
    await SetShiftReceiptUploadMode(mode);
    notify("Close-shift receipt photo setting updated");
  } catch {
    form.shiftReceiptUploadMode = previous;
    notify("Failed to update close-shift receipt photo setting", "red");
  } finally {
    shiftReceiptUploadModeSaving.value = false;
  }
}

async function setOpeningFloatEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.openingFloatEnabled;
  form.openingFloatEnabled = enabled;
  openingFloatEnabledSaving.value = true;
  try {
    await SetOpeningFloatEnabled(enabled);
    notify(`Opening cash float ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.openingFloatEnabled = previous;
    notify("Failed to update opening cash float setting", "red");
  } finally {
    openingFloatEnabledSaving.value = false;
  }
}

async function setCountOpeningCashOnClose(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.countOpeningCashOnClose;
  form.countOpeningCashOnClose = enabled;
  countOpeningCashOnCloseSaving.value = true;
  try {
    await SetCountOpeningCashOnCloseEnabled(enabled);
    notify(
      enabled
        ? "Close-shift count now includes the opening float"
        : "Close-shift count now excludes the opening float",
    );
  } catch {
    form.countOpeningCashOnClose = previous;
    notify("Failed to update close-shift opening float setting", "red");
  } finally {
    countOpeningCashOnCloseSaving.value = false;
  }
}

async function togglePageUnlock(page: string, value: boolean | null) {
  const enabled = Boolean(value);
  const previous = pageUnlocks.value[page];
  pageUnlocks.value = { ...pageUnlocks.value, [page]: enabled };
  pageUnlockSaving.value = true;
  try {
    await SetPageUnlocked(page, enabled);
    notify(`${pageUnlockLabels[page] ?? page} ${enabled ? "unlocked" : "locked"} for non-admin users`);
  } catch (error) {
    pageUnlocks.value = { ...pageUnlocks.value, [page]: Boolean(previous) };
    notify(error instanceof Error ? error.message : "Failed to update page access", "red");
  } finally {
    pageUnlockSaving.value = false;
  }
}

// kashi can flip this register's Quick Access source, or change its list,
// while this page is open.
const stopQuickAccessListener = EventsOn("sync_applied:quick_access", () => void loadQuickAccessItems());
onBeforeUnmount(stopQuickAccessListener);

onMounted(() => {
  // Fire every load concurrently rather than in a 14-deep await chain: they
  // have no ordering dependency, each already swallows its own errors (the
  // inline List* calls are wrapped below to match), and the slow OS
  // enumerations (ListPrinters especially) no longer sit in series in front
  // of the DB reads. settingsLoading is still driven by loadSettings, so the
  // progress bar clears as soon as the form data is in.
  const wrap = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
    } catch {}
  };
  void Promise.all([
    loadDatabaseStatus(),
    loadBranches(),
    loadCouponTypes(),
    loadSettings(),
    loadCurrencies(),
    loadPaymentMethodsList(),
    loadPriceLists(),
    loadDiscountLists(),
    loadUsers(),
    loadSalespersons(),
    loadCashierPin(),
    loadQuickAccessItems(),
    wrap(async () => {
      serialPorts.value = await ListSerialPorts();
    }),
    wrap(async () => {
      printers.value = await ListPrinters();
    }),
    wrap(async () => {
      inventories.value = await ListInventories();
    }),
    wrap(async () => {
      roles.value = await ListRoles();
    }),
    wrap(async () => {
      branchTargets.value = ((await ListBranchTargetsWithProgress()) ?? []) as typeof branchTargets.value;
    }),
  ]);
});

async function saveSettings() {
  saving.value = true;
  try {
    const receiptWidth = Number(form.receiptWidth);
    const receiptHeight = Number(form.receiptHeight);
    const taxRate = Number(form.taxRate);
    const exchangeRate = Number(form.exchangeRate);
    const maxOverpayment = Number(form.maxOverpayment);
    const maxOverpaymentCents = form.allowOverpayment && Number.isFinite(maxOverpayment) && maxOverpayment >= 0
      ? Math.round(maxOverpayment * 100)
      : 0;
    await UpdateSettings(
      form.branchName,
      form.branchCode.trim().toUpperCase(),
      form.apiKey,
      form.printerID,
      form.printerSize,
      form.receiptPrinter,
      form.screenPort,
      Number.isFinite(receiptWidth) && receiptWidth > 0 ? Math.round(receiptWidth) : 80,
      Number.isFinite(receiptHeight) && receiptHeight > 0 ? Math.round(receiptHeight) : 297,
      Number.isFinite(taxRate) ? Math.min(Math.max(taxRate, 0), 1) : 0,
      roundingModes.some((m) => m.value === form.roundingMode) ? form.roundingMode : "half_up",
      roundingCurrencies.value.some((item) => item.value === form.roundingCurrency)
        ? form.roundingCurrency
        : (roundingCurrencies.value[0]?.value ?? form.roundingCurrency),
      Boolean(form.receiptEnabled),
      Number(form.salesInventoryId) > 0 ? Number(form.salesInventoryId) : 0,
      Number.isFinite(exchangeRate) && exchangeRate > 0 ? Math.round(exchangeRate) : 90000,
      maxOverpaymentCents,
    );
    await SetAutoCloseStaleShiftsEnabled(Boolean(form.autoCloseStaleShiftsEnabled));
    await UpdateCentralAPIURL(form.centralApiUrl.trim());
    await UpdateReceiptSettings(
      form.marketName.trim(), form.marketPhone.trim(), form.branchPhone.trim(), form.marketDescription.trim(), form.returnPolicy.trim(),
      form.website.trim(), form.instagram.trim(), JSON.stringify(form.socialPlatforms ?? []), JSON.stringify(form.socialHandles ?? {}),
      form.registerName.trim(), form.receiptFont, form.receiptBodyFont,
      Number(form.receiptTitleSize), Number(form.receiptBodySize),
      Boolean(form.receiptCutoff),
    );
    await UpdateAttendanceConfig(form.akuvoxIP, form.akuvoxUsername, form.akuvoxPassword, Boolean(form.attendanceEnabled));
    if (form.attendanceApproverPassword.trim()) await UpdateAttendanceApproverPassword(form.attendanceApproverPassword.trim());
    await SetAttendanceDuplicateInterval(Math.max(0, Math.floor(Number(form.attendanceDuplicateIntervalSeconds) || 0)));
    await SetAttendanceCashierHistoryEnabled(Boolean(form.attendanceCashierHistory));
    const exchangeWindowHours =
      Math.max(0, Math.floor(Number(form.exchangeWindowDays) || 0)) * 24 +
      Math.max(0, Math.floor(Number(form.exchangeWindowHours) || 0));
    await SetExchangeWindowHours(exchangeWindowHours);
    await SetSplitDisplay(form.splitPrimaryCurrency, Boolean(form.splitPrimaryFirst), Math.max(0, Math.floor(Number(form.splitPrimaryStep) || 0)));
    notify("Settings saved");
  } catch {
    notify("Failed to save", "red");
  }
  saving.value = false;
}

const openingFloatDialog = ref(false);
const openingFloatPassword = ref("");
const openingFloatError = ref("");
const openingFloatSaving = ref(false);
const openingFloatNegative = computed(
  () => Number(form.defaultOpeningFloatUsd) < 0 || Number(form.defaultOpeningFloatLbp) < 0,
);

async function confirmOpeningFloatSave() {
  openingFloatSaving.value = true;
  openingFloatError.value = "";
  try {
    const usd = Number(form.defaultOpeningFloatUsd);
    const lbp = Number(form.defaultOpeningFloatLbp);
    await UpdateDefaultOpeningFloat(
      Number.isFinite(usd) && usd > 0 ? Math.round(usd * 100) : 0,
      Number.isFinite(lbp) && lbp > 0 ? Math.round(lbp) : 0,
      openingFloatPassword.value,
    );
    openingFloatDialog.value = false;
    openingFloatPassword.value = "";
    notify("Default opening float saved");
  } catch (error) {
    openingFloatError.value = error instanceof Error ? error.message : String(error);
  } finally {
    openingFloatSaving.value = false;
  }
}

async function printCloseShiftSample() {
  samplePrinting.value = true;
  try {
    await PrintCurrentCloseShiftSample();
    notify("Current close-shift sample sent to the printer");
  } catch (error) {
    notify(error instanceof Error ? error.message : "Could not print close-shift sample", "red");
  } finally {
    samplePrinting.value = false;
  }
}

async function loadSalespersons() {
  try {
    salespersons.value = await ListSalespersons();
  } catch {}
}

function openSalespersonDialog(salesperson?: any) {
  editingSalesperson.value = salesperson ?? null;
  salespersonName.value = salesperson?.fullName ?? "";
  salespersonAttendanceUserId.value = salesperson?.attendanceUserId ?? "";
  salespersonDialog.value = true;
}

async function saveSalesperson() {
  const editing = editingSalesperson.value;
  if (salespersonNameEditable.value && !salespersonName.value.trim()) return;
  savingSalesperson.value = true;
  try {
    // No editing target => creating a new local salesperson. Only possible
    // when central sync is off; with kashi sync on the roster is upstream-owned
    // and the Add button is hidden (see salespersonsSyncManaged).
    let id: number;
    if (!editing) {
      const created = await CreateSalesperson(salespersonName.value);
      id = Number(created.id);
    } else {
      id = editing.id;
      // Renaming is standalone-only -- with kashi connected the name is
      // kashi-owned, also for a till-only leftover (see salespersonNameEditable).
      if (salespersonNameEditable.value) {
        await UpdateSalesperson(id, salespersonName.value);
      }
    }
    await UpdateSalespersonAttendanceID(id, salespersonAttendanceUserId.value);
    salespersonDialog.value = false;
    notify(editing ? "Salesperson updated" : "Salesperson created");
    await loadSalespersons();
  } catch {
    notify("Failed to save salesperson", "red");
  } finally {
    savingSalesperson.value = false;
  }
}

function deleteSalespersonConfirm(salesperson: any) {
  deleteSalespersonTarget.value = salesperson;
  deleteSalespersonDialog.value = true;
}

async function deleteSalesperson() {
  try {
    await DeleteSalesperson(deleteSalespersonTarget.value.id);
    deleteSalespersonDialog.value = false;
    notify("Salesperson deleted");
    await loadSalespersons();
  } catch {
    notify("Could not delete salesperson", "red");
  }
}

async function changePassword() {
  if (passwordForm.newPass !== passwordForm.confirm) {
    notify("Passwords do not match", "red");
    return;
  }
  changingPassword.value = true;
  try {
    await ChangeAdminPassword(passwordForm.current, passwordForm.newPass);
    notify("Admin password changed");
    passwordForm.current = "";
    passwordForm.newPass = "";
    passwordForm.confirm = "";
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to change password", "red");
  }
  changingPassword.value = false;
}

async function loadCashierPin() {
  try {
    cashierPinCurrent.value = await GetCashierPIN();
  } catch {
    cashierPinCurrent.value = "";
  }
}

async function saveCashierPin() {
  if (!/^\d{4}$/.test(cashierPinForm.newPin)) {
    notify("PIN must be exactly 4 digits", "red");
    return;
  }
  if (cashierPinForm.newPin !== cashierPinForm.confirm) {
    notify("PINs do not match", "red");
    return;
  }
  savingCashierPin.value = true;
  try {
    await SetCashierPIN(cashierPinForm.newPin);
    cashierPinCurrent.value = cashierPinForm.newPin;
    cashierPinForm.newPin = "";
    cashierPinForm.confirm = "";
    notify("Cashier PIN updated");
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to update PIN", "red");
  }
  savingCashierPin.value = false;
}

async function loadUsers() {
  try {
    users.value = await ListUsers();
  } catch {}
}

function openUserDialog(user?: any) {
  editingUser.value = user ?? null;
  if (user) {
    userForm.username = user.username;
    userForm.password = "";
    userForm.roleId = user.roleId;
    userForm.attendanceUserId = user.attendanceUserId ?? "";
  } else {
    userForm.username = "";
    userForm.password = "";
    userForm.roleId = roles.value[0]?.id ?? 1;
    userForm.attendanceUserId = "";
  }
  userDialog.value = true;
}

async function saveUser() {
  savingUser.value = true;
  try {
    if (editingSyncedUser.value) {
      // Roster is kashi-owned -- keep every field as synced, change only the PIN.
      await UpdateUser(
        editingUser.value.id,
        editingUser.value.username,
        userForm.password,
        editingUser.value.roleId,
        editingUser.value.active ? 1 : 0,
      );
    } else if (editingUser.value) {
      await UpdateUser(
        editingUser.value.id,
        userForm.username,
        userForm.password,
        userForm.roleId,
        1,
      );
      await UpdateUserAttendanceID(editingUser.value.id, userForm.attendanceUserId.trim());
    } else {
      const created = await CreateUser(userForm.username, userForm.password, userForm.roleId);
      await UpdateUserAttendanceID(Number(created.id), userForm.attendanceUserId.trim());
    }
    notify(editingUser.value ? "User updated" : "User created");
    userDialog.value = false;
    await loadUsers();
  } catch {
    notify("Failed to save user", "red");
  }
  savingUser.value = false;
}

function deleteUserConfirm(user: any) {
  deleteTarget.value = user;
  deleteDialog.value = true;
}

async function deleteUser() {
  try {
    await DeleteUser(deleteTarget.value.id);
    notify("User deleted");
    deleteDialog.value = false;
    await loadUsers();
  } catch {
    notify("Failed to delete user", "red");
  }
}
</script>

<style scoped>
.salesperson-color-dot {
  display: inline-block;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  vertical-align: middle;
}
.qa-color-row {
  align-items: center;
  display: flex;
  gap: 12px;
  padding: 6px 0;
}
.qa-color-row + .qa-color-row {
  border-top: 1px solid rgba(var(--v-border-color), 0.12);
}
.qa-color-row__preview {
  border: 1px solid rgba(var(--v-border-color), 0.12);
  border-radius: 8px;
  flex: 0 0 160px;
  line-height: 1.2;
  min-width: 0;
  padding: 6px 8px;
}
.qa-color-row__price {
  opacity: var(--v-medium-emphasis-opacity);
}
.qa-color-row__pickers {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.qa-color-row__label {
  align-self: center;
  width: 36px;
}
.qa-swatch--auto {
  align-items: center;
  display: inline-flex;
  font-weight: 600;
  justify-content: center;
}
.qa-color-row__swatches {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.qa-swatch {
  border: 1px solid rgba(var(--v-border-color), 0.3);
  border-radius: 50%;
  cursor: pointer;
  height: 28px;
  width: 28px;
}
.qa-swatch--none {
  background: linear-gradient(
    to top right,
    transparent calc(50% - 1px),
    rgb(var(--v-theme-error)) calc(50% - 1px),
    rgb(var(--v-theme-error)) calc(50% + 1px),
    transparent calc(50% + 1px)
  );
}
.qa-swatch--selected {
  outline: 3px solid rgb(var(--v-theme-primary));
  outline-offset: 2px;
}
.target-color-swatch {
  display: inline-block;
  width: 14px;
  height: 14px;
  border-radius: 4px;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.2);
}

.settings-page {
  box-sizing: border-box;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.settings-header {
  flex: 0 0 auto;
  padding: 28px 32px 0;
}

.settings-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding: 20px 32px 40px;
}

.settings-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  max-width: 1200px;
  margin: 0 auto 20px;
}

.settings-search,
.settings-tabs,
.settings-grid,
.settings-empty {
  max-width: 1200px;
  margin-inline: auto;
}

.settings-search :deep(.v-field) {
  border-radius: 10px;
}

.settings-empty {
  padding: 32px 8px;
  text-align: center;
  color: rgb(var(--v-theme-on-surface), 0.6);
}

.settings-tabs {
  border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  min-height: 44px;
  margin-bottom: 4px;
}

.settings-tabs :deep(.v-tab) {
  min-width: 0;
  padding-inline: 12px;
  text-transform: none;
  font-weight: 600;
  letter-spacing: 0;
}

.settings-tabs :deep(.v-tab .v-btn__content) {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.settings-tab__icon {
  flex: 0 0 auto;
  opacity: 0.85;
}

.settings-tab__label {
  line-height: 1;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  align-items: start;
  gap: 16px;
}

.settings-page-actions {
  max-width: 1200px;
  margin: 16px auto 0;
  display: flex;
  justify-content: flex-end;
}

.settings-page :deep(.v-text-field),
.settings-page :deep(.v-select),
.settings-page :deep(.v-textarea) {
  --v-input-control-height: 40px;
}

/* The password card is wrapped in a <v-form> so the submit handler fires;
   display:contents keeps the form out of the grid layout so its child
   .settings-card is the actual grid item, sized like every other panel. */
.settings-form-wrap {
  display: contents;
}

.settings-card--wide {
  grid-column: 1 / -1;
}

.payment-method-color-swatch {
  display: inline-block;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1px solid rgba(var(--v-border-color), 0.3);
}

.payment-method-color-input {
  width: 40px;
  height: 36px;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
}

.settings-card {
  min-width: 0;
  background: transparent !important;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 12px;
  box-shadow: none !important;
}

.settings-card :deep(.v-card-title) {
  padding: 16px 18px 8px;
  font-size: var(--kashi-font-size-base) !important;
  line-height: 1.35;
  font-weight: 650;
}

.settings-card :deep(.v-card-text) {
  padding: 12px 18px 16px;
}

.settings-card :deep(.v-table) {
  background: transparent;
}

.field-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 12px 16px;
  align-items: start;
}

.field-grid > .field-full {
  grid-column: 1 / -1;
}

.app-snackbar .v-snackbar__wrapper {
  background: rgba(var(--v-theme-surface-variant), 0.72) !important;
  color: rgb(var(--v-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}

@media (max-width: 900px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }

  .settings-header {
    padding: 16px 16px 0;
  }

  .settings-scroll {
    padding: 12px 16px 24px;
  }
}
</style>
