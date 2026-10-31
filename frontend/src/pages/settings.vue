<template>
  <div class="settings-page">
    <div class="settings-header">
      <k-text-field
        v-model="settingsSearch"
        class="settings-search mb-2"
        placeholder="Search settings…"
        prepend-inner-icon="mdi-magnify"
        variant="outlined"
        density="compact"
        clearable
        hide-details
      />

      <k-tabs v-model="tab" class="settings-tabs" color="primary" grow>
        <k-tab v-for="t in settingsTabs" :key="t.value" :value="t.value" class="settings-tab">
          <k-icon :icon="t.icon" size="20" class="settings-tab__icon" />
          <span class="settings-tab__label">{{ t.label }}</span>
        </k-tab>
      </k-tabs>
      <k-progress-linear v-if="settingsLoading" indeterminate color="primary" />
    </div>

    <div class="settings-scroll">
      <div v-if="normalizedSearch && !tabsWithMatches?.size" class="settings-empty">
        No settings match "{{ settingsSearch }}".
      </div>

      <k-window v-model="tab">
      <k-window-item value="general">
        <div class="settings-grid">
          <k-card v-show="panelMatches('general-branch')" class="settings-card">
            <k-card-title>Branch</k-card-title>
            <k-card-text class="field-grid">
              <k-text-field v-model="form.branchName" placeholder="Branch name" variant="outlined" density="compact" hide-details />
              <k-text-field
                v-model="form.branchCode"
                placeholder="Branch code"
                maxlength="8"
                variant="outlined"
                density="compact"
                hide-details
                @update:model-value="form.branchCode = form.branchCode.toUpperCase()"
              />
              <k-select
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
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('general-branches')" class="settings-card">
            <k-card-title>Branches</k-card-title>
            <k-card-text>
              <k-select
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
                <k-btn
                  variant="text"
                  size="small"
                  :disabled="!activeBranchCode || activeBranchCode === defaultBranchCode"
                  :loading="settingDefaultBranch"
                  @click="setDefaultBranch(activeBranchCode)"
                >
                  Make active branch default
                </k-btn>
              </div>
              <k-divider class="my-4" />
              <div class="text-subtitle-2 mb-2">Add another branch</div>
              <k-text-field
                v-model="newBranch.name"
                label="Branch name"
                variant="outlined"
                density="compact"
                hide-details
                class="mb-2"
              />
              <k-text-field
                v-model="newBranch.code"
                label="Branch code"
                hint="Letters and numbers only"
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
                @update:model-value="newBranch.code = newBranch.code.toUpperCase()"
              />
              <k-text-field
                v-model="newBranch.apiKey"
                label="Central API key"
                variant="outlined"
                density="compact"
                hide-details
                class="mb-2"
              />
              <k-text-field
                v-model="newBranch.centralApiUrl"
                label="Central server URL"
                variant="outlined"
                density="compact"
                hide-details
                class="mb-2"
              />
              <div class="d-flex justify-end">
                <k-btn color="primary" variant="flat" :loading="creatingBranch" @click="createBranch">
                  Create branch
                </k-btn>
              </div>
              <k-alert v-if="branchActionError" type="error" density="compact" class="mt-3">
                {{ branchActionError }}
              </k-alert>

              <k-divider class="my-4" />
              <div class="text-subtitle-2 mb-1">Import branch history from kashi</div>
              <div class="text-caption text-medium-emphasis mb-2">For a reinstalled till with no history yet.</div>
              <k-btn
                variant="outlined"
                :loading="importingHistory"
                @click="importBranchHistory"
              >
                Import branch history
              </k-btn>
              <k-alert v-if="historyImportResult" type="success" density="compact" class="mt-3">
                Imported {{ historyImportResult.invoices }} invoice(s),
                {{ historyImportResult.expenses }} expense(s), {{ historyImportResult.loans }}
                loan(s) and {{ historyImportResult.shiftCloses }} shift close(s).
                <span v-if="historyImportResult.unresolvedItems">
                  {{ historyImportResult.unresolvedItems }} line item(s) used a placeholder
                  product.
                </span>
              </k-alert>
              <k-alert v-if="historyImportError" type="error" density="compact" class="mt-3">
                {{ historyImportError }}
              </k-alert>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('general-database')" class="settings-card">
            <k-card-title>Database</k-card-title>
            <k-card-text>
              <k-text-field
                :model-value="databasePath"
                label="Current database"
                variant="outlined"
                density="compact"
                readonly
                hide-details
              />
              <div class="d-flex justify-end mt-2">
                <k-btn variant="text" color="primary" :loading="databaseSwitching" @click="chooseDatabase">
                  <k-icon start icon="mdi-folder-open-outline" /> Choose database
                </k-btn>
              </div>
            </k-card-text>
          </k-card>
          <k-card v-if="cashMachine?.supported" v-show="panelMatches('general-cash-machine')" class="settings-card">
            <k-card-title>Cash machine</k-card-title>
            <k-card-text>
              <p v-if="!cashMachine.on" class="text-caption text-medium-emphasis mb-1">Off (kiosk mode is off).</p>
              <p v-if="cashMachine.closedPrograms.length" class="text-caption mb-1 settings-wrap">
                <strong>Closed:</strong>
                {{ cashMachine.closedPrograms.map((p) => (p.count > 1 ? `${p.name} (${p.count})` : p.name)).join(", ") }}
              </p>
              <p v-if="cashMachine.startupOff.length" class="text-caption mb-1 settings-wrap">
                <strong>Startup off:</strong> {{ cashMachine.startupOff.join(", ") }}
              </p>
              <template v-if="cashMachine.lastSetup">
                <p class="text-caption mt-2 mb-0">
                  Set up {{ new Date(cashMachine.lastSetup.at).toLocaleString() }}
                </p>
                <k-list density="compact" class="py-0">
                  <k-list-item v-for="s in cashMachine.lastSetup.steps" :key="s.name" :title="s.name" :subtitle="s.detail">
                    <template #prepend>
                      <k-icon :icon="s.ok ? 'mdi-check-circle' : 'mdi-close-circle'" :color="s.ok ? 'success' : 'error'" />
                    </template>
                  </k-list-item>
                </k-list>
              </template>
              <p v-if="cashMachine.setupRunning && cashMachine.setupStarted" class="text-caption mb-0">
                Running since {{ new Date(cashMachine.setupStarted).toLocaleTimeString() }}…
              </p>
              <div class="d-flex justify-end">
                <k-btn variant="text" color="primary" :loading="cashMachineRunning" @click="cashMachineConfirm = true">
                  <k-icon start icon="mdi-cash-register" /> Make this a cash machine
                </k-btn>
              </div>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('general-stuck-keys')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-keyboard" size="20" />
              Stuck keys
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.releaseStuckKeys"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Let go of Windows, Ctrl, Alt or Shift left held for 3 seconds"
                @update:model-value="setReleaseStuckKeys"
              />
              <p class="text-caption text-medium-emphasis mt-1 mb-0 settings-wrap">
                A remote session (AnyDesk from a Mac) can leave a key held down, so typing or scanning runs Windows
                shortcuts. Keys held on the till's own keyboard or scanner are never let go.
              </p>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('general-sync')" class="settings-card">
            <k-card-title>Central sync (kashi)</k-card-title>
            <k-card-text class="field-grid">
              <k-text-field
                v-model="form.apiKey"
                placeholder="API key"
                variant="outlined"
                density="compact"
                hide-details
                :append-inner-icon="showApiKey ? 'mdi-eye' : 'mdi-eye-off'"
                :type="showApiKey ? 'text' : 'password'"
                @click:append-inner="showApiKey = !showApiKey"
              />
              <k-text-field
                v-model="form.centralApiUrl"
                placeholder="Central server URL"
                variant="outlined"
                density="compact"
                hide-details
                class="field-full"
                :append-inner-icon="showCentralApiUrl ? 'mdi-eye' : 'mdi-eye-off'"
                :type="showCentralApiUrl ? 'text' : 'password'"
                @click:append-inner="showCentralApiUrl = !showCentralApiUrl"
              />

              <div class="field-full sync-everything">
                <k-checkbox
                  v-model="resyncApplyStock"
                  density="compact"
                  hide-details
                  label="Also apply stock movements to kashi"
                />
                <div class="d-flex align-center ga-2 mt-2">
                  <k-btn
                    color="primary"
                    variant="flat"
                    :loading="resyncRunning"
                    @click="runResync(false)"
                  >
                    Sync everything now
                  </k-btn>
                  <k-btn
                    variant="text"
                    :disabled="resyncRunning"
                    @click="runResync(true)"
                  >
                    Preview
                  </k-btn>
                  <span v-if="resyncProgress" class="text-caption text-medium-emphasis">
                    {{ resyncProgress }}
                  </span>
                </div>

                <k-alert
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
                </k-alert>

                <k-table v-if="resyncReport" density="compact" class="mt-3">
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
                </k-table>

                <k-table v-if="resyncReport?.fullCheck" density="compact" class="mt-3">
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
                </k-table>
              </div>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('general-targets')" class="settings-card">
            <k-card-title>Targets</k-card-title>
            <k-card-text>
              <div v-if="!branchTargets.length" class="text-caption text-medium-emphasis">
                No targets set for this branch.
              </div>
              <k-table v-else density="compact">
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
              </k-table>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('general-admin-legacy')" class="settings-card">
            <k-card-title>kashi-admin (legacy)</k-card-title>
            <k-card-text class="field-grid">
              <k-text-field v-model="form.adminListen" placeholder="Listen address" variant="outlined" density="compact" hide-details />
              <k-text-field
                v-model="form.adminToken"
                placeholder="Admin token"
                variant="outlined"
                density="compact"
                hide-details
                :type="showAdminToken ? 'text' : 'password'"
                :append-inner-icon="showAdminToken ? 'mdi-eye' : 'mdi-eye-off'"
                @click:append-inner="showAdminToken = !showAdminToken"
              />
              <k-switch v-model="form.adminEnabled" label="Enable remote admin API" color="primary" hide-details class="field-full" />
            </k-card-text>
          </k-card>
          <AppUpdateCard v-show="panelMatches('admin-app-updates')" />
        </div>
        <div class="settings-page-actions">
          <k-btn color="primary" variant="flat" :loading="saving || adminSettingsSaving" @click="saveGeneralSettings">
            <k-icon start icon="mdi-content-save-outline" /> Save
          </k-btn>
        </div>
      </k-window-item>

      <k-window-item value="money">
        <div class="settings-grid">
          <k-card v-show="panelMatches('money-rates')" class="settings-card">
            <k-card-title>Rates &amp; rounding</k-card-title>
            <k-card-text class="field-grid">
              <k-text-field v-model.number="form.taxRate" placeholder="VAT rate" type="number" step="0.01" min="0" max="1" variant="outlined" density="compact" hide-details />
              <k-text-field v-model.number="form.exchangeRate" placeholder="Legacy exchange rate" type="number" min="1" step="1" variant="outlined" density="compact" hide-details />
              <k-select v-model="form.roundingMode" :items="roundingModes" placeholder="Money rounding" variant="outlined" density="compact" hide-details />
              <k-select v-model="form.roundingCurrency" :items="roundingCurrencies" placeholder="Round by currency" variant="outlined" density="compact" hide-details />
              <k-select v-model="form.splitPrimaryCurrency" :items="roundingCurrencies" placeholder="Split primary currency" variant="outlined" density="compact" hide-details />
              <k-switch v-model="form.splitPrimaryFirst" color="primary" density="compact" hide-details :label="form.splitPrimaryFirst ? 'Primary currency first' : 'Rounding currency first'" />
              <k-text-field
                v-model.number="form.splitPrimaryStep"
                placeholder="Split step (0 = cash rounding)"
                type="number"
                min="0"
                step="1"
                variant="outlined"
                density="compact"
                hide-details
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('money-exchange')" class="settings-card">
            <k-card-title>Exchange settings</k-card-title>
            <k-card-text class="field-grid">
              <k-text-field v-model.number="form.exchangeWindowDays" label="Days" type="number" min="0" step="1" variant="outlined" density="compact" hide-details />
              <k-text-field v-model.number="form.exchangeWindowHours" label="Hours" type="number" min="0" max="23" step="1" variant="outlined" density="compact" hide-details />
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('money-currencies')" class="settings-card settings-card--wide">
            <k-card-title>Currencies &amp; rates</k-card-title>
            <k-card-text>
              <div class="settings-note">Set in kashi.</div>
              <k-table density="compact">
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
              </k-table>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('money-price-lists')" class="settings-card">
            <k-card-title>Price lists</k-card-title>
            <k-card-text>
              <div class="settings-note">Set in kashi. A product no active list prices can't be sold.</div>
              <k-table density="compact">
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
                      <k-chip v-if="list.isDefault" size="x-small" class="ml-2">default</k-chip>
                    </td>
                    <td>{{ list.isActive ? "Yes" : "No" }}</td>
                    <td>
                      <k-btn icon="mdi-format-list-bulleted-square" variant="text" size="small" title="View items" @click="openManageList('price', list)" />
                    </td>
                  </tr>
                  <tr v-if="!priceLists.length">
                    <td colspan="3" class="text-medium-emphasis">No price lists have synced yet.</td>
                  </tr>
                </tbody>
              </k-table>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('money-discount-lists')" class="settings-card">
            <k-card-title>Discount lists</k-card-title>
            <k-card-text>
              <div class="settings-note">Set in kashi.</div>
              <k-table density="compact">
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
                      <k-chip v-if="list.isDefault" size="x-small" class="ml-2">default</k-chip>
                    </td>
                    <td>{{ list.isActive ? "Yes" : "No" }}</td>
                    <td>
                      <k-btn icon="mdi-format-list-bulleted-square" variant="text" size="small" title="View items" @click="openManageList('discount', list)" />
                    </td>
                  </tr>
                  <tr v-if="!discountLists.length">
                    <td colspan="3" class="text-medium-emphasis">No discount lists have synced yet.</td>
                  </tr>
                </tbody>
              </k-table>
            </k-card-text>
          </k-card>
        </div>
        <div class="settings-page-actions">
          <k-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
            <k-icon start icon="mdi-content-save-outline" /> Save
          </k-btn>
        </div>
      </k-window-item>

      <k-window-item value="payments">
        <div class="settings-grid">
          <k-card v-show="panelMatches('payments-till')" class="settings-card">
            <k-card-title>At the till</k-card-title>
            <k-card-text class="field-grid">
              <k-text-field
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
              <k-switch v-model="form.allowOverpayment" label="Allow overpayment" color="primary" hide-details class="field-full" />
              <k-switch
                :model-value="form.autoAppendLbpZeros"
                :loading="autoAppendLbpZerosSaving"
                label="Auto-append 000 to LBP amounts"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setAutoAppendLbpZeros(value)"
              />
              <k-switch
                :model-value="form.overchangeConfirmationEnabled"
                :loading="overchangeConfirmationSaving"
                label="Confirm when giving more change than owed"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setOverchangeConfirmationEnabled(value)"
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('money-payment-methods')" class="settings-card settings-card--wide">
            <k-card-title>Payment methods</k-card-title>
            <k-card-text>
              <div class="settings-note">Set in kashi.</div>
              <k-table density="compact">
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
              </k-table>
              <k-switch
                :model-value="!form.hidePaymentMethodCurrency"
                :loading="paymentMethodSaving"
                color="primary"
                hide-details
                label="Show currency on payment method buttons"
                class="mt-2"
                @update:model-value="(value) => setHidePaymentMethodCurrency(!value)"
              />
            </k-card-text>
          </k-card>
        </div>
        <div class="settings-page-actions">
          <k-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
            <k-icon start icon="mdi-content-save-outline" /> Save
          </k-btn>
        </div>
      </k-window-item>

      <k-window-item value="coupons">
        <div class="settings-grid">
          <k-card v-show="panelMatches('coupons-types')" class="settings-card">
            <k-card-title>Coupon types</k-card-title>
            <k-card-text>
              <div class="d-flex flex-wrap ga-2 align-start">
                <k-text-field v-model="couponTypeForm.name" placeholder="Type name" variant="outlined" density="compact" style="max-width: 160px" />
                <k-text-field v-model.number="couponTypeForm.value" placeholder="Value" type="number" min="0.01" step="0.01" variant="outlined" density="compact" style="max-width: 110px" />
                <k-text-field v-model="couponTypeForm.currency" placeholder="Currency" maxlength="3" variant="outlined" density="compact" style="max-width: 100px" />
                <k-btn color="primary" :loading="couponTypeSaving" @click="saveCouponType">{{ editingCouponTypeId ? "Save" : "Add" }}</k-btn>
                <k-btn v-if="editingCouponTypeId" variant="text" @click="cancelCouponTypeEdit">Cancel</k-btn>
              </div>
              <div class="d-flex align-center ga-4 mb-2">
                <k-checkbox
                  v-model="couponTypeForm.codeRequired"
                  density="compact"
                  hide-details
                  label="Code required"
                  title="Uncheck for coupon types that need no code or reference number at all - they're added directly at checkout"
                />
                <k-checkbox v-model="couponTypeForm.referenceOnly" :disabled="!couponTypeForm.codeRequired" density="compact" hide-details label="Reference-only" />
                <k-checkbox v-model="couponTypeForm.isActive" density="compact" hide-details label="Active" />
                <k-checkbox
                  v-model="couponTypeForm.calculateOnCheckout"
                  density="compact"
                  hide-details
                  label="Don't count in totals (settled later)"
                  title="Tick when the coupon is settled later (e.g. reimbursed by the issuer) instead of collected by the branch now. Its payments are left out of the shift's total and net sales."
                />
              </div>
              <k-list density="compact" bg-color="transparent">
                <k-list-item v-for="couponType in couponTypes" :key="couponType.id" :title="couponType.name" :subtitle="`${(Number(couponType.valueCents) / 100).toFixed(2)} ${couponType.currencyCode}${couponType.codeRequired === false ? ' · No code required' : couponType.referenceOnly ? ' · Reference-only' : ''}${couponType.calculateOnCheckout ? ' · Not counted in totals' : ''}`">
                  <template #append>
                    <k-btn icon="mdi-pencil-outline" variant="text" density="compact" title="Edit coupon type" @click="editCouponType(couponType)" />
                    <k-btn icon="mdi-delete-outline" variant="text" density="compact" title="Delete coupon type" @click="removeCouponType(couponType)" />
                  </template>
                </k-list-item>
                <k-list-item v-if="!couponTypes.length" title="No coupon types configured" />
              </k-list>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('coupons-issued')" class="settings-card">
            <k-card-title>Issued coupons</k-card-title>
            <k-card-text>
              <div class="d-flex flex-wrap ga-2 align-start mb-2">
                <k-text-field v-model="couponCodeForm" placeholder="New coupon code" variant="outlined" density="compact" />
                <k-select v-model="couponIssueTypeId" :items="couponTypes" item-title="name" item-value="id" placeholder="Type" variant="outlined" density="compact" />
                <k-btn color="secondary" :loading="couponIssuing" @click="issueCoupon">Issue</k-btn>
              </div>
              <k-list density="compact" bg-color="transparent">
                <k-list-item v-for="coupon in coupons" :key="coupon.code" :title="coupon.code" :subtitle="`${(Number(coupon.valueCents) / 100).toFixed(2)} · ${coupon.status}`">
                  <template #append><k-btn icon="mdi-delete-outline" variant="text" density="compact" title="Delete coupon" @click="removeCoupon(coupon)" /></template>
                </k-list-item>
                <k-list-item v-if="!coupons.length" title="No coupons issued" />
              </k-list>
            </k-card-text>
          </k-card>
        </div>
      </k-window-item>

      <k-window-item value="behaviour">
        <div class="settings-grid">
          <k-card v-show="panelMatches('behavior-custom-pricing')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-sale-outline" size="20" />
              Custom item pricing
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.customItemPricesEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Custom item prices"
                @update:model-value="setCustomPricesEnabled"
              />
              <k-switch
                :model-value="form.perUnitItemPricesEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Per-unit price/discount editing"
                @update:model-value="setPerUnitPricesEnabled"
              />
              <k-switch
                :model-value="form.customItemDiscountsEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Custom product discounts"
                @update:model-value="setCustomDiscountsEnabled"
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-price-change-mode')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-tag-multiple-outline" size="20" />
              Price change mode
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.priceChangeManualOverrideEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Manual override (independent price & discount)"
                @update:model-value="setPriceChangeManualOverrideEnabled"
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-product-search')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-magnify" size="20" />
              Product search button
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.productSearchEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Show the product search button on the sale screen"
                @update:model-value="setProductSearchEnabled"
              />
              <k-switch
                :model-value="productSearchInPlace"
                color="primary"
                hide-details
                label="Search in the sale screen: results in place of the calculator, keyboard along the bottom (off: a full-screen dialog)"
                @update:model-value="(value) => setProductSearchInPlace(value ?? false)"
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-search-code-only')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-barcode-scan" size="20" />
              Code-only product search
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.productSearchCodeOnly"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Search products by code only and hide their names"
                @update:model-value="setProductSearchCodeOnly"
              />

            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-stock')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-package-variant-closed" size="20" />
              Selling beyond stock
            </k-card-title>
            <k-card-text>
              <k-select
                v-model="form.oversellMode"
                :items="oversellModes"
                :loading="pricingSettingsSaving"
                density="compact"
                hide-details
                label="Selling more than the till has in stock"
                variant="outlined"
                @update:model-value="saveOversellMode"
              />
<div class="settings-note mt-1">kashi alerts on any stock below zero.</div>
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-search-columns')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-magnify" size="20" />
              Product search &amp; unknown item
            </k-card-title>
            <k-card-text>
              <k-select
                v-model="form.productSearchHiddenFields"
                :items="productSearchFieldOptions"
                chips
                closable-chips
                density="compact"
                hide-details
                label="Hide in product search"
                multiple
                variant="outlined"
                @update:model-value="saveProductSearchHiddenFields"
              />
              <k-text-field
                v-model="form.unknownProductBarcode"
                class="mt-3"
                density="compact"
                hide-details
                label="Unknown item barcode (empty = built-in)"
                variant="outlined"
                @blur="saveUnknownProductBarcode"
              />
              <k-select
                :model-value="form.unknownPicturesAtCheckout"
                :items="unknownPicturesModeOptions"
                class="mt-3"
                density="compact"
                hide-details
                label="Unknown item pictures QR code"
                variant="outlined"
                @update:model-value="setUnknownPicturesAtCheckout"
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-price-edit-codes')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-tag-multiple-outline" size="20" />
              Price edits by code
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.priceEditRestricted"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Allow manual price and discount edits only for selected codes"
                @update:model-value="setPriceEditRestricted"
              />
              <k-textarea
                v-model="form.priceEditCodes"
                class="mt-3"
                label="Allowed codes"
                placeholder="One per line, or separated by commas"
                variant="outlined"
                density="compact"
                rows="3"
                auto-grow
                hide-details
                :disabled="!form.priceEditRestricted"
                @blur="savePriceEditCodes"
              />
              <div class="settings-note mt-1">Needs custom prices or discounts on.</div>
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-discount-toggle')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-tag-off-outline" size="20" />
              Turn discounts or deals off at the till
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.discountToggleEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Salespersons can tap a line's discount (30%) to turn it off or on"
                @update:model-value="setDiscountToggleEnabled"
              />
              <k-switch
                :model-value="form.discountToggleDefaultOff"
                :loading="pricingSettingsSaving"
                :disabled="!form.discountToggleEnabled"
                color="primary"
                hide-details
                label="Discounts start off on each new line"
                @update:model-value="setDiscountToggleDefaultOff"
              />
              <k-switch
                :model-value="form.dealToggleEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Salespersons can tap a line's deal (B2G1) to turn it off or on"
                @update:model-value="setDealToggleEnabled"
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-scan-by-code')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-barcode-scan" size="20" />
              Product code in scan field
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.scanByCodeEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Accept a product code in the scan field"
                @update:model-value="setScanByCodeEnabled"
              />
              <div class="settings-note mt-1">Used when no barcode matches.</div>
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-client-required')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-account-off-outline" size="20" />
              Client on checkout
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.clientRequired"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Require selecting a client before checkout"
                @update:model-value="setClientRequired"
              />
              <div class="settings-note mt-1">Off: invoices default to Anonymous.</div>
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-salesperson-per-invoice')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-account-tie-outline" size="20" />
              Salesperson per invoice
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.salespersonPerInvoiceRequired"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Require picking a salesperson on every invoice"
                @update:model-value="setSalespersonPerInvoiceRequired"
              />
              <div class="settings-note mt-1">Off: the last salesperson carries over.</div>
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-non-admin-month-nav')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-calendar-month-outline" size="20" />
              Income chart month navigation
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.nonAdminMonthNavEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Let non-admins browse other months on the income chart"
                @update:model-value="setNonAdminMonthNavEnabled"
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-visitor-counter')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-account-group-outline" size="20" />
              People visits counter
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.visitorCounterEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Show the visits counter in the side menu"
                @update:model-value="setVisitorCounterEnabled"
              />
              <div class="settings-note mt-1">Turning it off keeps recorded counts.</div>
            </k-card-text>
          </k-card>

        </div>
      </k-window-item>

      <k-window-item value="display">
        <div class="settings-grid">
          <k-card v-show="panelMatches('display-look')" class="settings-card">
            <k-card-title>Look &amp; feel</k-card-title>
            <k-card-text class="field-grid">
              <k-switch
                :model-value="darkThemeEnabled"
                :loading="themeSaving"
                :disabled="themeSaving"
                label="Dark theme"
                color="primary"
                hide-details
                @update:model-value="setDarkTheme"
              />
              <k-switch
                :model-value="animationsEnabled"
                label="Animations"
                color="primary"
                hide-details
                @update:model-value="(value) => setAnimationsEnabled(value ?? false)"
              />
              <k-select
                :model-value="calculatorTotalCurrency"
                :items="calculatorDisplayCurrencies"
                placeholder="Calculator total currency"
                variant="outlined"
                density="compact"
                hide-details
                @update:model-value="(value) => setCalculatorTotalCurrency(value ?? 'USD')"
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('behavior-sounds')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-volume-high" size="20" />
              Sounds
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.soundsEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Play sounds"
                @update:model-value="setSoundsEnabled"
              />
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('behavior-invoice-keyboard')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-keyboard" size="20" />
              Invoice keyboard mode
            </k-card-title>
            <k-card-text>
              <k-switch
                :model-value="form.invoiceKeyboardModeEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Enable keyboard mode"
                @update:model-value="setInvoiceKeyboardModeEnabled"
              />
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('display-quick-access-cards')" class="settings-card">
            <k-card-title>Quick access cards</k-card-title>
            <k-card-text>
              <div class="text-body-2">
                Quick access rows:
                <strong>{{ quickAccessRows === 0 ? "Hidden" : quickAccessRows }}</strong>
              </div>
              <k-slider
                :model-value="quickAccessRows"
                :min="0"
                :max="QUICK_ACCESS_MAX_ROWS"
                :step="1"
                color="primary"
                show-ticks="always"
                hide-details
                @update:model-value="setQuickAccessRows"
              />
              <div class="text-body-2 mt-2">
                Quick access text size: <strong>{{ quickAccessFontScale }}%</strong>
              </div>
              <k-slider
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
                  <k-btn
                    size="small"
                    variant="text"
                    :disabled="quickAccessFontScale === 100"
                    @click="setQuickAccessFontScale(100)"
                  >
                    Reset
                  </k-btn>
                </template>
              </k-slider>
              <k-switch
                :model-value="quickAccessShowCode"
                label="Show code on quick access cards"
                color="primary"
                hide-details
                @update:model-value="(value) => setQuickAccessShowCode(value ?? false)"
              />
              <k-switch
                :model-value="quickAccessShowName"
                label="Show name on quick access cards"
                color="primary"
                hide-details
                @update:model-value="(value) => setQuickAccessShowName(value ?? false)"
              />
              <k-switch
                :model-value="quickAccessShowRemove"
                label="Show remove (X) button on quick access cards"
                color="primary"
                hide-details
                @update:model-value="(value) => setQuickAccessShowRemove(value ?? false)"
              />
            </k-card-text>
          </k-card>

        </div>
      </k-window-item>

      <k-window-item value="quick_access">
        <div class="settings-grid">
          <k-card v-show="panelMatches('general-quick-access-colors')" class="settings-card settings-card--wide">
            <k-card-title class="d-flex align-center">
              Quick access colors
              <k-spacer />
              <k-btn
                variant="tonal"
                size="small"
                prepend-icon="mdi-download"
                :loading="exportingQuickAccess"
                :disabled="!quickAccessItems.length"
                @click="exportQuickAccessCsv"
              >
                Export CSV
              </k-btn>
            </k-card-title>
            <k-card-text>
              <div class="d-flex align-center flex-wrap ga-3 mb-3">
                <span class="text-body-2">Cards come from</span>
                <k-btn-toggle
                  :model-value="quickAccessMode"
                  mandatory
                  density="compact"
                  variant="outlined"
                  color="primary"
                  :disabled="!auth.isAdmin || changingQuickAccessMode"
                  @update:model-value="changeQuickAccessMode"
                >
                  <k-btn size="small" value="till">This till</k-btn>
                  <k-btn size="small" value="kashi">kashi</k-btn>
                </k-btn-toggle>
                <span v-if="quickAccessMode === 'kashi'" class="text-caption text-medium-emphasis">
                  Edit in kashi: Branch settings → Quick access.
                </span>
                <span v-else-if="!quickAccessKashiAvailable" class="text-caption text-medium-emphasis">
                  kashi isn't connected.
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
            </k-card-text>
          </k-card>
        </div>
      </k-window-item>

      <k-window-item value="cash">
        <div class="settings-grid">
          <k-card v-show="panelMatches('money-float')" class="settings-card">
            <k-card-title>Opening cash float</k-card-title>
            <k-card-text class="field-grid">
              <k-switch
                :model-value="form.openingFloatEnabled"
                :loading="openingFloatEnabledSaving"
                label="Enable opening cash float"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setOpeningFloatEnabled(value)"
              />
              <template v-if="form.openingFloatEnabled">
                <k-text-field v-model.number="form.defaultOpeningFloatUsd" placeholder="Opening float (USD)" type="number" min="0" step="0.01" variant="outlined" density="compact" hide-details />
                <k-text-field v-model.number="form.defaultOpeningFloatLbp" placeholder="Opening float (LBP)" type="number" min="0" step="1" variant="outlined" density="compact" hide-details />
                <k-alert v-if="openingFloatNegative" type="warning" variant="tonal" density="compact" class="field-full">
                  Opening float cannot be negative.
                </k-alert>
                <k-btn color="primary" variant="tonal" :disabled="openingFloatNegative" class="field-full" @click="openingFloatDialog = true">Save opening float</k-btn>
                <k-switch
                  :model-value="form.countOpeningCashOnClose"
                  :loading="countOpeningCashOnCloseSaving"
                  label="Count opening float in close-shift cash count"
                  color="primary"
                  hide-details
                  class="field-full"
                  @update:model-value="(value) => setCountOpeningCashOnClose(value)"
                />
              </template>
            </k-card-text>
          </k-card>
          <k-card v-show="panelMatches('cash-closing')" class="settings-card">
            <k-card-title>Closing shifts</k-card-title>
            <k-card-text class="field-grid">
              <k-switch
                v-model="form.autoCloseStaleShiftsEnabled"
                label="Auto-close previous-day shifts"
                color="primary"
                hide-details
                class="field-full"
              />
              <k-switch
                :model-value="form.closeShiftRequiresSync"
                :loading="closeShiftRequiresSyncSaving"
                label="Upload everything before closing a shift"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setCloseShiftRequiresSync(value)"
              />
              <div class="settings-note field-full">Offline, the admin password overrides.</div>
              <k-switch
                :model-value="form.shiftReceiptUploadMode !== 0"
                :loading="shiftReceiptUploadModeSaving"
                label="Show receipt-photo QR code after closing a shift"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setShiftReceiptUploadMode(value ? 2 : 0)"
              />
              <template v-if="form.shiftReceiptUploadMode !== 0">
                <k-select
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
                <div class="settings-note field-full">
                  Hybrid: office when online, else this till. Local: this till. Central: office.
                </div>
              </template>
              <k-switch
                :model-value="form.autoCoverNegativeLbp"
                :loading="autoCoverNegativeLbpSaving"
                label="At close shift, cover a negative Cash LL balance from Cash $$"
                color="primary"
                hide-details
                class="field-full"
                @update:model-value="(value) => setAutoCoverNegativeLbp(value)"
              />
              <div class="settings-note field-full">So Cash LL closes at 0.</div>
            </k-card-text>
          </k-card>

        </div>
        <div class="settings-page-actions">
          <k-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
            <k-icon start icon="mdi-content-save-outline" /> Save
          </k-btn>
        </div>
      </k-window-item>

      <k-window-item value="printing">
        <div class="settings-grid">
          <k-card v-show="panelMatches('printing-receipt')" class="settings-card">
            <k-card-title>Receipt</k-card-title>
            <k-card-text class="field-grid">
              <k-select v-model="form.printerSize" :items="['58mm', '80mm']" placeholder="Printing size" variant="outlined" density="compact" hide-details />
              <k-text-field v-model.number="form.receiptWidth" placeholder="Width (mm)" type="number" variant="outlined" density="compact" hide-details />
              <k-text-field v-model.number="form.receiptHeight" placeholder="Height (mm)" type="number" variant="outlined" density="compact" hide-details />
              <k-select v-model="form.receiptFont" :items="['Belleza', 'Helvetica', 'Times', 'Courier']" placeholder="Title font" variant="outlined" density="compact" hide-details />
              <k-select v-model="form.receiptBodyFont" :items="['Printer Font A', 'Courier', 'Helvetica', 'Times', 'Belleza']" placeholder="Body font" variant="outlined" density="compact" hide-details />
              <k-text-field v-model.number="form.receiptTitleSize" placeholder="Market name size" type="number" min="8" max="40" variant="outlined" density="compact" hide-details />
              <k-text-field v-model.number="form.receiptBodySize" placeholder="Body size" type="number" min="6" max="24" variant="outlined" density="compact" hide-details />
              <k-switch v-model="form.receiptEnabled" label="Print receipts after checkout" color="primary" hide-details class="field-full" />
              <k-switch v-model="form.receiptCutoff" label="Cut paper after printing" color="primary" hide-details class="field-full" />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('printing-look')" class="settings-card settings-card--wide">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-palette" size="20" />
              Receipt look
            </k-card-title>
            <k-card-text>
              <ReceiptLookPicker @saved="onReceiptLookSaved" @error="(message: string) => notify(message, 'red')" />
              <div class="settings-note mt-2">Fonts and sizes above apply to the Classic theme.</div>
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('printing-end-of-shift')" class="settings-card">
            <k-card-title>End-of-shift receipt</k-card-title>
            <k-card-text class="field-grid">
              <k-switch v-model="form.shiftReportSignatures" label="Cashier and manager signature lines" color="primary" hide-details class="field-full" />
              <k-switch v-model="form.shiftReportUppercaseHeader" label="Store and branch name in capitals" color="primary" hide-details class="field-full" />
            </k-card-text>
            <k-card-actions class="px-4 pb-4">
              <k-btn
                variant="tonal"
                color="primary"
                :loading="samplePrinting"
                @click="printCloseShiftSample"
              >
                <k-icon start icon="mdi-printer-eye" /> Print sample
              </k-btn>
            </k-card-actions>
          </k-card>

          <k-card v-show="panelMatches('printing-header-footer')" class="settings-card">
            <k-card-title>Receipt header &amp; footer</k-card-title>
            <k-card-text class="field-grid">
              <k-text-field v-model="form.marketName" placeholder="Market name" variant="outlined" density="compact" hide-details />
              <k-text-field v-model="form.marketPhone" placeholder="Market phone" variant="outlined" density="compact" hide-details />
              <k-text-field v-model="form.marketDescription" placeholder="Tagline" variant="outlined" density="compact" hide-details />
              <k-text-field v-model="form.branchPhone" placeholder="Branch phone" variant="outlined" density="compact" hide-details />
              <k-text-field v-model="form.registerName" placeholder="Register name (POS-01)" variant="outlined" density="compact" hide-details />
              <k-text-field v-model="form.website" placeholder="Website" variant="outlined" density="compact" hide-details />
              <k-textarea v-model="form.returnPolicy" placeholder="Return policy" rows="2" variant="outlined" density="compact" hide-details class="field-full" />
              <k-select
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
                  <k-chip v-bind="props" :icon="platformIcon(String(item.value))" label variant="tonal" />
                </template>
                <template #item="{ props, item }">
                  <k-list-item v-bind="props" :prepend-icon="platformIcon(String(item.value))">
                    <template #title>
                      <span class="sr-only">{{ platformLabel(String(item.value)) }}</span>
                    </template>
                  </k-list-item>
                </template>
              </k-select>
              <k-text-field
                v-for="platform in form.socialPlatforms"
                :key="platform"
                v-model="form.socialHandles[platform]"
                :placeholder="`${platformLabel(platform)} handle`"
                :prepend-inner-icon="platformIcon(platform)"
                variant="outlined"
                density="compact"
                hide-details
              />
            </k-card-text>
          </k-card>

          <k-card v-show="panelMatches('printing-devices')" class="settings-card">
            <k-card-title>Devices</k-card-title>
            <k-card-text class="field-grid">
              <k-select v-model="form.receiptPrinter" :items="printers" placeholder="Receipt printer" variant="outlined" density="compact" hide-details />
              <k-select v-model="form.screenPort" :items="serialPorts" placeholder="Customer screen COM port" variant="outlined" density="compact" hide-details />
            </k-card-text>
          </k-card>

        </div>
        <div class="settings-page-actions">
          <k-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
            <k-icon start icon="mdi-content-save-outline" /> Save
          </k-btn>
        </div>
      </k-window-item>

      <k-window-item value="attendance">
        <div class="settings-grid">
          <k-card v-show="panelMatches('attendance-machine')" class="settings-card">
            <k-card-title>Attendance machine</k-card-title>
            <k-card-text class="field-grid">
              <k-text-field v-model="form.akuvoxIP" placeholder="Akuvox IP address" variant="outlined" density="compact" hide-details />
              <k-text-field v-model="form.akuvoxUsername" placeholder="Device username" variant="outlined" density="compact" hide-details />
              <k-text-field v-model="form.akuvoxPassword" placeholder="Device password" type="password" variant="outlined" density="compact" hide-details />
              <k-text-field v-model="form.attendanceApproverPassword" placeholder="Manager password" type="password" variant="outlined" density="compact" hide-details />
              <k-text-field v-model.number="form.attendanceDuplicateIntervalSeconds" placeholder="Duplicate interval (sec)" type="number" min="0" step="1" variant="outlined" density="compact" hide-details />
              <k-switch v-model="form.attendanceEnabled" label="Enable attendance" color="primary" hide-details class="field-full" />
              <k-switch v-model="form.attendanceCashierHistory" label="Cashiers can browse previous days" color="primary" hide-details class="field-full" />
              <k-switch
                v-model="form.attendanceManagerApproval"
                label="Manager approves requests and adds missed punches"
                color="primary"
                hide-details
                class="field-full"
              />
            </k-card-text>
          </k-card>
        </div>
        <div class="settings-page-actions">
          <k-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
            <k-icon start icon="mdi-content-save-outline" /> Save
          </k-btn>
        </div>
      </k-window-item>

      <k-window-item value="users">
        <div class="settings-grid">
          <k-card v-show="panelMatches('admin-users')" class="settings-card settings-card--wide">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-account-group-outline" size="20" />
              Users
              <k-spacer />
              <k-btn color="primary" size="small" @click="openUserDialog()">
                <k-icon start icon="mdi-plus" /> Add user
              </k-btn>
            </k-card-title>
            <div class="settings-note px-4">kashi users: only their PIN can be reset here.</div>
            <k-data-table
              :headers="userHeaders"
              :items="users"
              items-per-page="-1"
              hide-default-footer
              density="comfortable"
              hover
            >
              <template #item.username="{ item }">
                {{ item.username }}
                <k-chip
                  v-if="item.isSynced"
                  class="ml-2"
                  color="primary"
                  size="x-small"
                  variant="tonal"
                >
                  kashi
                </k-chip>
              </template>
              <template #item.active="{ item }">
                <k-chip :color="item.active ? 'green' : 'red'" size="small" variant="tonal">
                  {{ item.active ? "Active" : "Disabled" }}
                </k-chip>
              </template>
              <template #item.actions="{ item }">
                <k-btn
                  v-if="item.isSynced"
                  icon
                  size="small"
                  variant="text"
                  title="Reset PIN"
                  @click="openUserDialog(item)"
                >
                  <k-icon icon="mdi-key-outline" size="18" />
                </k-btn>
                <template v-else>
                  <k-btn icon size="small" variant="text" @click="openUserDialog(item)">
                    <k-icon icon="mdi-pencil-outline" size="18" />
                  </k-btn>
                  <k-btn icon size="small" variant="text" @click="deleteUserConfirm(item)">
                    <k-icon icon="mdi-delete-outline" size="18" color="red" />
                  </k-btn>
                </template>
              </template>
            </k-data-table>
          </k-card>
          <k-card v-show="panelMatches('admin-salespersons')" class="settings-card settings-card--wide">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-account-tie" size="20" />
              Salespersons
              <k-spacer />
              <k-btn
                v-if="!salespersonsSyncManaged"
                color="primary"
                size="small"
                @click="openSalespersonDialog()"
              >
                <k-icon start icon="mdi-plus" /> Add salesperson
              </k-btn>
            </k-card-title>
            <div v-if="salespersonsSyncManaged" class="settings-note px-4">
              Set in kashi; only the attendance ID can be changed here.
            </div>
            <k-data-table
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
                <k-chip v-if="item.isSynced" :color="item.isActive ? 'green' : 'red'" size="small" variant="tonal">
                  {{ item.isActive ? "Active" : "Inactive" }}
                </k-chip>
                <span v-else class="text-caption text-medium-emphasis">Not synced</span>
              </template>
              <template #item.actions="{ item }">
                <k-btn icon size="small" variant="text" @click="openSalespersonDialog(item)">
                  <k-icon icon="mdi-pencil-outline" size="18" />
                </k-btn>
                <k-btn
                  v-if="!item.isSynced"
                  icon
                  size="small"
                  variant="text"
                  @click="deleteSalespersonConfirm(item)"
                >
                  <k-icon icon="mdi-delete-outline" size="18" color="red" />
                </k-btn>
              </template>
            </k-data-table>
          </k-card>
          <k-form v-show="panelMatches('admin-change-password')" class="settings-form-wrap" @submit.prevent="changePassword">
            <k-card class="settings-card">
              <k-card-title class="d-flex align-center ga-3">
                <k-icon icon="mdi-lock-reset" size="20" />
                Change admin password
              </k-card-title>
              <k-card-text>
                <k-text-field
                  v-model="passwordForm.current"
                  placeholder="Current admin password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                  :append-inner-icon="showPw ? 'mdi-eye' : 'mdi-eye-off'"
                  @click:append-inner="showPw = !showPw"
                />
                <k-text-field
                  v-model="passwordForm.newPass"
                  placeholder="New password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                />
                <k-text-field
                  v-model="passwordForm.confirm"
                  placeholder="Confirm new password"
                  variant="outlined"
                  density="compact"
                  :type="showPw ? 'text' : 'password'"
                  :rules="[(v) => v === passwordForm.newPass || 'Passwords do not match']"
                />
              </k-card-text>
              <k-card-actions>
                <k-spacer />
                <k-btn color="primary" variant="flat" type="submit" :loading="changingPassword">
                  <k-icon start icon="mdi-lock-reset" /> Change password
                </k-btn>
              </k-card-actions>
            </k-card>
          </k-form>
          <k-form v-show="panelMatches('admin-cashier-pin')" class="settings-form-wrap" @submit.prevent="saveCashierPin">
            <k-card class="settings-card">
              <k-card-title class="d-flex align-center ga-3">
                <k-icon icon="mdi-dialpad" size="20" />
                Cashier login PIN
              </k-card-title>
              <k-card-text>
                <k-text-field
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
                <k-text-field
                  v-model="cashierPinForm.confirm"
                  placeholder="Confirm PIN"
                  variant="outlined"
                  density="compact"
                  inputmode="numeric"
                  maxlength="4"
                  :type="showCashierPin ? 'text' : 'password'"
                  :rules="[(v) => v === cashierPinForm.newPin || 'PINs do not match']"
                />
              </k-card-text>
              <k-card-actions>
                <k-spacer />
                <k-btn
                  color="primary"
                  variant="flat"
                  type="submit"
                  :loading="savingCashierPin"
                  :disabled="!/^\d{4}$/.test(cashierPinForm.newPin) || cashierPinForm.newPin !== cashierPinForm.confirm"
                >
                  <k-icon start icon="mdi-dialpad" /> Update PIN
                </k-btn>
              </k-card-actions>
            </k-card>
          </k-form>
        </div>
      </k-window-item>

      <k-window-item value="pages">
        <div class="settings-grid">
          <k-card v-show="panelMatches('admin-page-access')" class="settings-card">
            <k-card-title class="d-flex align-center ga-3">
              <k-icon icon="mdi-lock-outline" size="20" />
              Page access
            </k-card-title>
            <k-card-text>
              <k-switch
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
            </k-card-text>
          </k-card>
        </div>
      </k-window-item>
      </k-window>
    </div>

    <k-dialog v-model="userDialog" max-width="400">
      <k-card>
        <k-card-title>
          {{ editingSyncedUser ? "Reset PIN" : editingUser ? "Edit User" : "New User" }}
        </k-card-title>
        <k-card-text>
          <div v-if="editingSyncedUser" class="text-body-2 text-medium-emphasis mb-3">
            {{ editingUser.username }} is managed from kashi. Only the PIN can be reset here.
          </div>
          <k-text-field
            v-if="!editingSyncedUser"
            v-model="userForm.username"
            placeholder="Username"
            variant="outlined"
            density="compact"
            class="mb-2"
          />
          <k-text-field
            v-model="userForm.password"
            variant="outlined"
            density="compact"
            class="mb-2"
            :type="showUpw ? 'text' : 'password'"
            :append-inner-icon="showUpw ? 'mdi-eye' : 'mdi-eye-off'"
            @click:append-inner="showUpw = !showUpw"
            :placeholder="editingSyncedUser ? 'New PIN' : editingUser ? 'Leave blank to keep current' : 'Password'"
          />
          <k-select
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
          <k-text-field
            v-if="!editingSyncedUser"
            v-model="userForm.attendanceUserId"
            placeholder="Akuvox UserID (leave blank for visitors)"
            variant="outlined"
            density="compact"
          />
        </k-card-text>
        <k-card-actions>
          <k-btn variant="text" @click="userDialog = false">Cancel</k-btn>
          <k-spacer />
          <k-btn
            color="primary"
            variant="flat"
            :disabled="editingSyncedUser ? !userForm.password : !userForm.username"
            :loading="savingUser"
            @click="saveUser"
            >Save</k-btn
          >
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog :model-value="showDeactivationConfirm" max-width="480" persistent>
      <k-card v-if="pendingDeactivation">
        <k-card-title>Deactivate products no longer on kashi?</k-card-title>
        <k-card-text>
          <div class="mb-2">
            kashi no longer has these. They'll be hidden from checkout here too (past invoices keep them).
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
        </k-card-text>
        <k-card-actions>
          <k-btn variant="text" :disabled="deactivationConfirming" @click="cancelDeactivation">Cancel</k-btn>
          <k-spacer />
          <k-btn color="primary" variant="flat" :loading="deactivationConfirming" @click="confirmDeactivationAndResync">
            Deactivate and sync
          </k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog v-model="cashMachineConfirm" max-width="520">
      <k-card>
        <k-card-title>Make this a cash machine?</k-card-title>
        <k-card-text>
          <p class="mb-2">Windows will ask for an administrator's approval. Then, on this computer:</p>
          <ul class="ml-5 mb-2">
            <li>Startup apps, third-party scheduled tasks and background services are switched off.</li>
            <li>Apps a till doesn't use are removed: OneDrive, Teams, Xbox, Copilot, widgets, news, games and the like.</li>
            <li>Notifications, tips and ads are off; the computer never sleeps.</li>
            <li>Windows Update only restarts outside 7:00-23:00, never during working hours.</li>
          </ul>
          <p>Removed apps don't come back by themselves. It takes a few minutes and runs in the background.</p>
        </k-card-text>
        <k-card-actions>
          <k-btn variant="text" @click="cashMachineConfirm = false">Cancel</k-btn>
          <k-spacer />
          <k-btn color="primary" variant="flat" @click="setUpCashMachine">Make it a cash machine</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog v-model="deleteDialog" max-width="400">
      <k-card>
        <k-card-title>Delete User</k-card-title>
        <k-card-text>Are you sure you want to delete {{ deleteTarget?.username }}?</k-card-text>
        <k-card-actions>
          <k-btn variant="text" @click="deleteDialog = false">Cancel</k-btn>
          <k-spacer />
          <k-btn color="red" variant="flat" @click="deleteUser">Delete</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog v-model="salespersonDialog" max-width="400">
      <k-card>
        <k-card-title>
          {{
            !salespersonNameEditable
              ? "Attendance ID"
              : editingSalesperson
                ? "Edit Salesperson"
                : "New Salesperson"
          }}
        </k-card-title>
        <k-card-text>
          <k-text-field
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
          <k-text-field v-model="salespersonAttendanceUserId" placeholder="Akuvox attendance ID" variant="outlined" density="compact" />
        </k-card-text>
        <k-card-actions>
          <k-btn variant="text" @click="salespersonDialog = false">Cancel</k-btn>
          <k-spacer />
          <k-btn
            color="primary"
            variant="flat"
            :disabled="salespersonNameEditable && !salespersonName.trim()"
            :loading="savingSalesperson"
            @click="saveSalesperson"
            >Save</k-btn
          >
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog v-model="deleteSalespersonDialog" max-width="400">
      <k-card>
        <k-card-title>Delete Salesperson</k-card-title>
        <k-card-text>Delete {{ deleteSalespersonTarget?.fullName }}?</k-card-text>
        <k-card-actions>
          <k-btn variant="text" @click="deleteSalespersonDialog = false">Cancel</k-btn>
          <k-spacer />
          <k-btn color="red" variant="flat" @click="deleteSalesperson">Delete</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog v-model="manageListDialog" max-width="520">
      <k-card>
        <k-card-title>{{ manageListName }} items</k-card-title>
        <k-card-text>
          <k-table density="compact">
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
          </k-table>
        </k-card-text>
        <k-card-actions>
          <k-spacer />
          <k-btn variant="text" @click="manageListDialog = false">Close</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-dialog v-model="openingFloatDialog" max-width="380">
      <k-card>
        <k-card-title>Save opening float</k-card-title>
        <k-card-text>
          <div class="text-body-2 mb-3">
            Applies to shifts opened after saving. Requires the manager password.
          </div>
          <div class="text-caption text-medium-emphasis mb-1">Manager password</div>
          <k-text-field
            v-model="openingFloatPassword"
            placeholder="••••••••"
            type="password"
            variant="outlined"
            density="compact"
            autofocus
            @keyup.enter="confirmOpeningFloatSave"
          />
          <k-alert v-if="openingFloatError" type="error" variant="tonal" density="compact">{{ openingFloatError }}</k-alert>
        </k-card-text>
        <k-card-actions>
          <k-spacer />
          <k-btn variant="text" @click="openingFloatDialog = false">Cancel</k-btn>
          <k-btn color="primary" :loading="openingFloatSaving" @click="confirmOpeningFloatSave">Save</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>

    <k-snackbar v-model="snackbar.show" class="app-snackbar" timeout="3500" location="top" variant="flat">
      {{ snackbar.text }}
    </k-snackbar>
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
  SetProductSearchCodeOnly,
  SetUnknownPicturesAtCheckout,
  SetPriceEditRestricted,
  SetDiscountToggleEnabled,
  SetDiscountToggleDefaultOff,
  SetDealToggleEnabled,
  SetOversellMode,
  SetPriceEditCodes,
  SetProductSearchHiddenFields,
  SetSoundsEnabled,
  SetReleaseStuckKeys,
  SetUnknownProductBarcode,
  SetVisitorCounterEnabled,
  SetClientRequired,
  SetSalespersonPerInvoiceRequired,
  SetNonAdminMonthNavEnabled,
  SetHidePaymentMethodCurrency,
  SetAutoAppendLbpZerosEnabled,
  SetOverchangeConfirmationEnabled,
  SetCloseShiftRequiresSyncEnabled,
  SetOpeningFloatEnabled,
  SetShiftReportSignaturesEnabled,
  SetShiftReportUppercaseHeaderEnabled,
  SetCountOpeningCashOnCloseEnabled,
  SetAutoCoverNegativeLBPEnabled,
  SetShiftReceiptUploadMode,
  SetPageUnlocked,
  UpdateDefaultOpeningFloat,
  SetAutoCloseStaleShiftsEnabled,
  SetAttendanceCashierHistoryEnabled,
  SetAttendanceManagerApprovalEnabled,
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
  GetCashMachineStatus,
  StartCashMachineSetup,
} from "../../wailsjs/go/main/App";
import { EventsOn, WindowReloadApp } from "../../wailsjs/runtime/runtime";
import type { main } from "../../wailsjs/go/models";
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
import ReceiptLookPicker from "@/components/ReceiptLookPicker.vue";
import {
  animationsEnabled,
  setAnimationsEnabled,
  productSearchInPlace,
  setProductSearchInPlace,
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
import { useTheme } from "@/ui";

const auth = useAuthStore();
const { setClientRequired: setClientRequiredState } = useClient();
const { setSalespersonPerInvoiceRequired: setSalespersonPerInvoiceRequiredState } = useSalesperson();
const { refresh: refreshVisitorCounter } = useVisitorCounter();
const theme = useTheme();
const tab = ref("general");

const settingsTabs = [
  { value: "general", label: "General", icon: "mdi-cog-outline" },
  { value: "money", label: "Money", icon: "mdi-cash-multiple" },
  { value: "payments", label: "Payments", icon: "mdi-credit-card-outline" },
  { value: "coupons", label: "Coupons", icon: "mdi-ticket-percent-outline" },
  { value: "behaviour", label: "Behaviour", icon: "mdi-button-pointer" },
  { value: "display", label: "Till display", icon: "mdi-monitor-dashboard" },
  { value: "quick_access", label: "Quick access", icon: "mdi-flash" },
  { value: "cash", label: "Cash & shifts", icon: "mdi-cash-register" },
  { value: "printing", label: "Printing", icon: "mdi-printer-outline" },
  { value: "attendance", label: "Attendance", icon: "mdi-clipboard-check-outline" },
  { value: "users", label: "Users", icon: "mdi-account-group-outline" },
  { value: "pages", label: "Page access", icon: "mdi-lock-outline" },
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
  "money-rates": { tab: "money", title: "Rates & rounding", keywords: "vat tax rate exchange rate rounding mode currency split primary step cash rounding" },
  "payments-till": { tab: "payments", title: "At the till", keywords: "overpayment max allow auto append lbp zeros 000 thousands confirm change overchange refund more than owed" },
  "display-look": { tab: "display", title: "Look & feel", keywords: "dark theme animations calculator total currency shows first mode display" },
  "display-quick-access-cards": { tab: "display", title: "Quick access cards", keywords: "quick access rows text size font bigger smaller show code name remove x button cards" },
  "cash-closing": { tab: "cash", title: "Closing shifts", keywords: "close shift auto close previous day stale upload before closing wait pending offline receipt photo qr hybrid local central cover negative cash ll lbp swap" },
  "printing-end-of-shift": { tab: "printing", title: "End-of-shift receipt", keywords: "end of shift close shift signature cashier manager capitals uppercase header sample print" },
  "printing-header-footer": { tab: "printing", title: "Receipt header & footer", keywords: "market name phone tagline description branch phone register name website return policy social platforms instagram facebook handle" },
  "printing-look": { tab: "printing", title: "Receipt look", keywords: "receipt look theme layout style design columns preview classic supermarket boutique modern bistro typewriter fonts" },
  "printing-devices": { tab: "printing", title: "Devices", keywords: "receipt printer customer screen com port serial" },
  "general-quick-access-colors": { tab: "quick_access", title: "Quick access colors", keywords: "quick access card color colour background text font source kashi central managed till" },
  "general-branch": { tab: "general", title: "Branch", keywords: "branch name code sales inventory" },
  "general-database": { tab: "general", title: "Database", keywords: "database path choose switch" },
  "general-branches": { tab: "general", title: "Branches", keywords: "branches switch default active add branch multiple instances" },
  "general-cash-machine": { tab: "general", title: "Cash machine", keywords: "cash machine windows performance slow programs apps startup close kiosk minimal onedrive teams" },
  "general-stuck-keys": { tab: "general", title: "Stuck keys", keywords: "stuck keys windows key win ctrl alt shift held anydesk remote mac keyboard scanner shortcuts" },
  "general-sync": { tab: "general", title: "Central sync (kashi)", keywords: "central sync kashi server url branch code api key upload before close shift wait pending offline" },
  "general-targets": { tab: "general", title: "Targets", keywords: "targets revenue goal progress date range" },
  "general-admin-legacy": { tab: "general", title: "kashi-admin (legacy)", keywords: "remote admin api listen address token legacy tui" },
  "attendance-machine": { tab: "attendance", title: "Attendance machine", keywords: "akuvox ip username password manager duplicate interval enable cashier history auto close stale shifts" },
  "printing-receipt": { tab: "printing", title: "Receipt", keywords: "receipt printer size width height print after checkout cut paper title font body font size" },
  "money-float": { tab: "cash", title: "Opening cash float", keywords: "opening cash float usd lbp drawer shift close count reconciliation expected enable disable feature toggle balance" },
  "money-exchange": { tab: "money", title: "Exchange settings", keywords: "exchange window days hours return" },
  "money-currencies": { tab: "money", title: "Currencies & rates", keywords: "currency code name symbol units per usd cash rounding unit active add currency" },
  "money-payment-methods": { tab: "payments", title: "Payment methods", keywords: "payment method cashbox account currency" },
  "money-price-lists": { tab: "money", title: "Price lists", keywords: "price list override product code black friday active" },
  "money-discount-lists": { tab: "money", title: "Discount lists", keywords: "discount list override product code percent active" },
  "coupons-types": { tab: "coupons", title: "Coupon types", keywords: "coupon type value currency reference only active" },
  "coupons-issued": { tab: "coupons", title: "Issued coupons", keywords: "coupon issue code new" },
  "behavior-custom-pricing": { tab: "behaviour", title: "Custom item pricing", keywords: "custom item price discount per unit editing" },
  "behavior-price-change-mode": { tab: "behaviour", title: "Price change mode", keywords: "price change manual override discount lever" },
  "behavior-invoice-keyboard": { tab: "display", title: "Invoice keyboard mode", keywords: "invoice keyboard qty discount price number pad" },
  "behavior-product-search": { tab: "behaviour", title: "Product search button", keywords: "product search magnifier sale screen find lookup disable hide dialog in place calculator keyboard" },
  "behavior-scan-by-code": { tab: "behaviour", title: "Product code in scan field", keywords: "barcode scan code sku variant type enter" },
  "behavior-search-code-only": { tab: "behaviour", title: "Code-only product search", keywords: "product search code only hide names barcode lookup" },
  "behavior-stock": { tab: "behaviour", title: "Selling beyond stock", keywords: "oversell stock allow warn block sell more than in stock negative" },
  "behavior-search-columns": { tab: "behaviour", title: "Product search & unknown item", keywords: "product search hide columns code name brand color size price unknown item barcode placeholder pictures qr phone checkout" },
  "behavior-discount-toggle": { tab: "behaviour", title: "Turn discounts or deals off at the till", keywords: "discount deal toggle turn off on remove salesperson default off b2g1 percent" },
  "behavior-price-edit-codes": { tab: "behaviour", title: "Price edits by code", keywords: "price discount manual edit restrict allow only selected codes products" },
  "behavior-sounds": { tab: "display", title: "Sounds", keywords: "sound audio beep chime click mute volume quiet silent" },
  "behavior-client-required": { tab: "behaviour", title: "Client on every invoice", keywords: "require client select checkout invoice anonymous prefer not to say" },
  "behavior-always-anonymous": { tab: "behaviour", title: "Always Anonymous client", keywords: "always anonymous client toggle button prefer not to say" },
  "behavior-salesperson-per-invoice": { tab: "behaviour", title: "Salesperson per invoice", keywords: "salesperson per invoice require force clear carry forward previous cashier pick" },
  "behavior-non-admin-month-nav": { tab: "behaviour", title: "Income chart month navigation", keywords: "monthly income chart non-admin cashier back forward prev next month browse lock admin only" },
  "behavior-visitor-counter": { tab: "behaviour", title: "People visits counter", keywords: "people visits counter footfall traffic side menu nav drawer up down orange chip monthly income" },
  "admin-app-updates": { tab: "general", title: "App updates", keywords: "update upgrade version install roll back rollback previous release download" },
  "admin-page-access": { tab: "pages", title: "Page access", keywords: "page access lock unlock non-admin clients inventory transfers attendance salespersons permissions" },
  "admin-change-password": { tab: "users", title: "Change admin password", keywords: "change admin password current new confirm account security login sign in" },
  "admin-cashier-pin": { tab: "users", title: "Cashier login PIN", keywords: "cashier pin code login keypad 4 digit change shared" },
  "admin-users": { tab: "users", title: "Users", keywords: "users username role status add user edit delete akuvox userid" },
  "admin-salespersons": { tab: "users", title: "Salespersons", keywords: "salespersons full name attendance id add edit delete" },
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
  returnPolicy: "Exchange within 3 days if items are unworn, unwashed and with tags.\nPlease present this receipt.",
  website: "yourwebsite.com",
  instagram: "@your_instagram",
  socialPlatforms: [] as string[],
  socialHandles: {} as Record<string, string>,
  receiptFont: "Belleza",
  receiptTitleSize: 21,
  receiptBodySize: 11,
  receiptBodyFont: "Printer Font A",
  receiptCutoff: true,
  shiftReportSignatures: true,
  shiftReportUppercaseHeader: false,
  customItemDiscountsEnabled: false,
  customItemPricesEnabled: false,
  perUnitItemPricesEnabled: false,
  priceChangeManualOverrideEnabled: false,
  invoiceKeyboardModeEnabled: false,
  productSearchEnabled: true,
  scanByCodeEnabled: true,
  productSearchCodeOnly: false,
  priceEditRestricted: false,
  priceEditCodes: "",
  discountToggleEnabled: false,
  discountToggleDefaultOff: false,
  dealToggleEnabled: false,
  oversellMode: "allow",
  productSearchHiddenFields: [] as string[],
  unknownProductBarcode: "",
  unknownPicturesAtCheckout: false,
  soundsEnabled: false,
  releaseStuckKeys: true,
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
  autoCoverNegativeLbp: false,
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
  attendanceManagerApproval: true,
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
const autoCoverNegativeLbpSaving = ref(false);
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
const showCentralApiUrl = ref(false);
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
        shiftReportSignatures: s.shiftReportSignatures !== false,
        shiftReportUppercaseHeader: Boolean(s.shiftReportUppercaseHeader),
        customItemDiscountsEnabled: Boolean(s.customItemDiscountsEnabled),
        customItemPricesEnabled: Boolean(s.customItemPricesEnabled),
        perUnitItemPricesEnabled: Boolean(s.perUnitItemPricesEnabled),
        priceChangeManualOverrideEnabled: Boolean(s.priceChangeManualOverrideEnabled),
        invoiceKeyboardModeEnabled: Boolean(s.invoiceKeyboardModeEnabled),
        productSearchEnabled: s.productSearchEnabled !== false,
        scanByCodeEnabled: s.scanByCodeEnabled !== false,
        productSearchCodeOnly: Boolean(s.productSearchCodeOnly),
        priceEditRestricted: Boolean(s.priceEditRestricted),
        discountToggleEnabled: Boolean(s.discountToggleEnabled),
        discountToggleDefaultOff: Boolean(s.discountToggleDefaultOff),
        dealToggleEnabled: Boolean(s.dealToggleEnabled),
        priceEditCodes: String(s.priceEditCodes ?? "").split(",").join("\n"),
        oversellMode: String(s.oversellMode || "allow"),
        productSearchHiddenFields: String(s.productSearchHiddenFields ?? "").split(",").filter(Boolean),
        unknownProductBarcode: String(s.unknownProductBarcode ?? ""),
        unknownPicturesAtCheckout: Boolean(s.unknownPicturesAtCheckout),
        soundsEnabled: s.soundsEnabled === true,
        releaseStuckKeys: s.releaseStuckKeys !== false,
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
        autoCoverNegativeLbp: Boolean(s.autoCoverNegativeLbp),
        shiftReceiptUploadMode: Number(s.shiftReceiptUploadMode ?? 2),
        akuvoxIP: s.attendance?.ip ?? "",
        akuvoxUsername: s.attendance?.username ?? "",
        attendanceEnabled: Boolean(s.attendance?.enabled),
        attendanceApproverPassword: "",
        attendanceDuplicateIntervalSeconds: Number(s.attendance?.duplicateIntervalSeconds ?? 30),
        attendanceCashierHistory: Boolean(s.attendance?.cashierHistory),
        attendanceManagerApproval: s.attendanceManagerApprovalEnabled !== false,
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

const cashMachine = ref<main.CashMachineStatus | null>(null);
const cashMachineConfirm = ref(false);
// The setup runs in the background (StartCashMachineSetup), so whether it's
// running comes from the till, not this page: leave and come back and the
// spinner is still there. CashMachineToast says when it's done.
const cashMachineRunning = computed(() => cashMachine.value?.setupRunning ?? false);

async function loadCashMachine() {
  try {
    cashMachine.value = await GetCashMachineStatus();
  } catch {}
}

async function setUpCashMachine() {
  cashMachineConfirm.value = false;
  try {
    await StartCashMachineSetup();
    notify("Making this a cash machine. Approve Windows' prompt; you can keep using the till.", "info");
  } catch (error) {
    notify(error instanceof Error ? error.message : String(error), "red");
  }
  await loadCashMachine();
}

const stopCashMachineListener = EventsOn("cash_machine_setup_done", () => void loadCashMachine());
onBeforeUnmount(stopCashMachineListener);

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

async function setProductSearchCodeOnly(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.productSearchCodeOnly;
  form.productSearchCodeOnly = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetProductSearchCodeOnly(enabled);
    notify(`Code-only product search ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.productSearchCodeOnly = previous;
    notify("Failed to update the product search setting", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setDiscountToggleEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.discountToggleEnabled;
  form.discountToggleEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetDiscountToggleEnabled(enabled);
    notify(`Turning discounts off at the till ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.discountToggleEnabled = previous;
    notify("Failed to update the discount toggle setting", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setDiscountToggleDefaultOff(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.discountToggleDefaultOff;
  form.discountToggleDefaultOff = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetDiscountToggleDefaultOff(enabled);
    notify(`Discounts start off ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.discountToggleDefaultOff = previous;
    notify("Failed to update the discount default setting", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setDealToggleEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.dealToggleEnabled;
  form.dealToggleEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetDealToggleEnabled(enabled);
    notify(`Turning deals off at the till ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.dealToggleEnabled = previous;
    notify("Failed to update the deal toggle setting", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setPriceEditRestricted(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.priceEditRestricted;
  form.priceEditRestricted = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetPriceEditRestricted(enabled);
    notify(`Price edits limited to selected codes ${enabled ? "enabled" : "disabled"}`);
  } catch {
    form.priceEditRestricted = previous;
    notify("Failed to update the price edit setting", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function savePriceEditCodes() {
  pricingSettingsSaving.value = true;
  try {
    await SetPriceEditCodes(form.priceEditCodes);
    notify("Allowed price edit codes saved");
  } catch {
    notify("Failed to save the allowed codes", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

const oversellModes = [
  { title: "Allow: sell, fix stock in kashi later", value: "allow" },
  { title: "Warn: cashier confirms before selling", value: "warn" },
  { title: "Block: refuse the sale", value: "block" },
];

const productSearchFieldOptions = [
  { title: "Code", value: "code" },
  { title: "Name", value: "name" },
  { title: "Brand", value: "brand" },
  { title: "Color", value: "color" },
  { title: "Size", value: "size" },
  { title: "Price", value: "price" },
];

async function saveOversellMode(value: string) {
  pricingSettingsSaving.value = true;
  try {
    await SetOversellMode(value);
    notify("Oversell mode saved");
  } catch {
    notify("Failed to save the oversell mode", "red");
    await loadSettings();
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function saveProductSearchHiddenFields(value: string[]) {
  try {
    await SetProductSearchHiddenFields(value.join(","));
    notify("Product search columns saved");
  } catch {
    notify("Failed to save the product search columns", "red");
    await loadSettings();
  }
}

const unknownPicturesModeOptions = [
  { title: "Right after adding an unknown item", value: false },
  { title: "At checkout", value: true },
];

async function setUnknownPicturesAtCheckout(value: boolean) {
  const previous = form.unknownPicturesAtCheckout;
  form.unknownPicturesAtCheckout = value;
  try {
    await SetUnknownPicturesAtCheckout(value);
    notify(value ? "Unknown item QR code shows at checkout" : "Unknown item QR code shows right after adding one");
  } catch {
    form.unknownPicturesAtCheckout = previous;
    notify("Failed to save the unknown item pictures setting", "red");
  }
}

async function saveUnknownProductBarcode() {
  try {
    await SetUnknownProductBarcode(form.unknownProductBarcode);
    notify("Unknown item product saved");
  } catch {
    notify("Failed to save the unknown item product", "red");
  }
}

function onReceiptLookSaved() {
  notify("Receipt look saved");
}

async function setSoundsEnabled(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.soundsEnabled;
  form.soundsEnabled = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetSoundsEnabled(enabled);
    notify(`Sounds ${enabled ? "on" : "off"}`);
  } catch {
    form.soundsEnabled = previous;
    notify("Failed to update the sounds setting", "red");
  } finally {
    pricingSettingsSaving.value = false;
  }
}

async function setReleaseStuckKeys(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.releaseStuckKeys;
  form.releaseStuckKeys = enabled;
  pricingSettingsSaving.value = true;
  try {
    await SetReleaseStuckKeys(enabled);
    notify(`Letting go of stuck keys ${enabled ? "on" : "off"}`);
  } catch {
    form.releaseStuckKeys = previous;
    notify("Failed to update the stuck keys setting", "red");
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

async function setAutoCoverNegativeLbp(value: boolean | null) {
  const enabled = Boolean(value);
  const previous = form.autoCoverNegativeLbp;
  form.autoCoverNegativeLbp = enabled;
  autoCoverNegativeLbpSaving.value = true;
  try {
    await SetAutoCoverNegativeLBPEnabled(enabled);
    notify(enabled ? "A negative Cash LL will be covered from Cash $$ at close" : "Negative Cash LL is left as it is at close");
  } catch {
    form.autoCoverNegativeLbp = previous;
    notify("Failed to update the negative LBP setting", "red");
  } finally {
    autoCoverNegativeLbpSaving.value = false;
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
    loadCashMachine(),
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
    await SetShiftReportSignaturesEnabled(Boolean(form.shiftReportSignatures));
    await SetShiftReportUppercaseHeaderEnabled(Boolean(form.shiftReportUppercaseHeader));
    await UpdateAttendanceConfig(form.akuvoxIP, form.akuvoxUsername, form.akuvoxPassword, Boolean(form.attendanceEnabled));
    if (form.attendanceApproverPassword.trim()) await UpdateAttendanceApproverPassword(form.attendanceApproverPassword.trim());
    await SetAttendanceDuplicateInterval(Math.max(0, Math.floor(Number(form.attendanceDuplicateIntervalSeconds) || 0)));
    await SetAttendanceCashierHistoryEnabled(Boolean(form.attendanceCashierHistory));
    await SetAttendanceManagerApprovalEnabled(Boolean(form.attendanceManagerApproval));
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
  border-top: 1px solid rgba(var(--k-border-color), 0.12);
}
.qa-color-row__preview {
  border: 1px solid rgba(var(--k-border-color), 0.12);
  border-radius: 8px;
  flex: 0 0 160px;
  line-height: 1.2;
  min-width: 0;
  padding: 6px 8px;
}
.qa-color-row__price {
  opacity: var(--k-medium-emphasis-opacity);
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
  gap: 4px;
}
.qa-swatch {
  border: 1px solid rgba(var(--k-border-color), 0.3);
  border-radius: 50%;
  cursor: pointer;
  height: 24px;
  width: 24px;
}
.qa-swatch--none {
  background: linear-gradient(
    to top right,
    transparent calc(50% - 1px),
    rgb(var(--k-theme-error)) calc(50% - 1px),
    rgb(var(--k-theme-error)) calc(50% + 1px),
    transparent calc(50% + 1px)
  );
}
.qa-swatch--selected {
  outline: 3px solid rgb(var(--k-theme-primary));
  outline-offset: 2px;
}
.target-color-swatch {
  display: inline-block;
  width: 14px;
  height: 14px;
  border-radius: 4px;
  border: 1px solid rgba(var(--k-theme-on-surface), 0.2);
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
  padding: 16px 24px 0;
}

.settings-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 24px 0;
}

/* A tab with no Save bar still gets room under its last card. */
.settings-grid:last-child {
  margin-bottom: 24px;
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

.settings-search :deep(.k-field) {
  border-radius: 10px;
}

.settings-empty {
  padding: 32px 8px;
  text-align: center;
  color: rgb(var(--k-theme-on-surface), 0.6);
}

.settings-tabs {
  border-bottom: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
  min-height: 44px;
  margin-bottom: 4px;
}

.settings-tabs :deep(.k-tab) {
  min-width: 0;
  padding-inline: 12px;
  text-transform: none;
  font-weight: 600;
  letter-spacing: 0;
  gap: 8px;
}

.settings-tab__icon {
  flex: 0 0 auto;
  opacity: 0.85;
}

.settings-tab__label {
  line-height: 1;
}

/* Cards stack in columns (masonry), so a short card doesn't leave the
   empty space a row of equal-height grid cells did. */
.settings-grid {
  columns: 2 340px;
  column-gap: 12px;
}

.settings-grid > .settings-card,
.settings-grid > .settings-form-wrap > .settings-card {
  break-inside: avoid;
  margin-bottom: 12px;
  width: 100%;
}

.settings-page-actions {
  max-width: 1200px;
  margin: 4px auto 0;
  display: flex;
  justify-content: flex-end;
  /* Stays at the bottom of the screen while the settings scroll under it. */
  position: sticky;
  bottom: 0;
  z-index: 5;
  padding: 10px 0 14px;
  background: var(--kashi-app-background);
}

.settings-page :deep(.k-input) {
  --k-ic-h: 40px;
}

/* The password card is wrapped in a <k-form> so the submit handler fires;
   display:contents keeps the form out of the grid layout so its child
   .settings-card is the actual grid item, sized like every other panel. */
.settings-form-wrap {
  display: contents;
}

.settings-card--wide {
  column-span: all;
}

.payment-method-color-swatch {
  display: inline-block;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1px solid rgba(var(--k-border-color), 0.3);
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
  border: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
  border-radius: 12px;
  box-shadow: none !important;
}

.settings-card :deep(.k-card-title) {
  padding: 10px 14px 2px;
  font-size: var(--kashi-font-size-base) !important;
  line-height: 1.35;
  font-weight: 650;
}

.settings-card :deep(.k-card-text) {
  padding: 8px 14px 12px;
}

.settings-card :deep(.k-card-actions) {
  padding: 0 8px 8px;
  min-height: 0;
}

/* Switches sit in a compact stack rather than 56px rows. */
.settings-card :deep(.k-switch .k-selection) {
  min-height: 36px;
}

/* One short grey line under a setting, in place of a paragraph. */
.settings-note {
  font-size: 0.75rem;
  line-height: 1.3;
  color: rgba(var(--k-theme-on-surface), 0.6);
  margin-bottom: 6px;
}

.settings-wrap {
  overflow-wrap: anywhere;
}

.settings-card :deep(.k-table) {
  background: transparent;
}

.field-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 8px 12px;
  align-items: start;
}

.field-grid > .field-full {
  grid-column: 1 / -1;
}

.app-snackbar .k-snackbar__wrapper {
  background: rgba(var(--k-theme-surface-variant), 0.72) !important;
  color: rgb(var(--k-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--k-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}

@media (max-width: 900px) {
  .settings-grid {
    columns: 1;
  }

  .settings-header {
    padding: 16px 16px 0;
  }

  .settings-scroll {
    padding: 12px 16px 0;
  }
}
</style>
