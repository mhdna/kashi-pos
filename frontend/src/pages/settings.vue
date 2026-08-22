<template>
  <div class="settings-page">
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

    <div v-if="normalizedSearch && !tabsWithMatches?.size" class="settings-empty">
      No settings match "{{ settingsSearch }}".
    </div>

    <v-tabs v-model="tab" class="settings-tabs mb-5" color="primary" grow>
      <v-tab value="general">General Settings</v-tab>
      <v-tab value="printing">Printing &amp; Receipt</v-tab>
      <v-tab value="attendance">Attendance</v-tab>
      <v-tab value="money">Money</v-tab>
      <v-tab value="coupons">Coupons</v-tab>
      <v-tab value="admin">Passwords &amp; Users</v-tab>
    </v-tabs>
    <v-progress-linear v-if="settingsLoading" indeterminate color="primary" class="mb-4" />

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
                hint="Turn off decorative motion for a simpler, faster display."
                persistent-hint
                hide-details
                class="mt-3"
                @update:model-value="(value) => setAnimationsEnabled(value ?? false)"
              />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('general-branch')" class="settings-card">
            <v-card-title>Branch</v-card-title>
            <v-card-text>
              <v-text-field
                v-model="form.branchName"
                placeholder="Branch name"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-text-field
                v-model="form.branchCode"
                placeholder="Branch code"
                hint="Used in invoice numbers, for example CN."
                maxlength="8"
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
                @update:model-value="form.branchCode = form.branchCode.toUpperCase()"
              />
              <v-select
                v-model.number="form.salesInventoryId"
                :items="inventories"
                item-title="name"
                item-value="id"
                placeholder="Sales inventory"
                hint="Stock is deducted from this inventory."
                persistent-hint
                variant="outlined"
                density="compact"
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

          <v-card v-show="panelMatches('general-remote')" class="settings-card">
            <v-card-title>Remote admin &amp; API</v-card-title>
            <v-card-text>
              <v-switch v-model="form.adminEnabled" label="Enable remote admin API" color="primary" hide-details class="mb-3" />
              <v-text-field v-model="form.adminListen" placeholder="Listen address" hint="Use 0.0.0.0:8787 for LAN access." persistent-hint variant="outlined" density="compact" class="mb-2" />
              <v-text-field
                v-model="form.adminToken"
                placeholder="Admin token / HTTP password"
                hint="Required when listening beyond localhost."
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
                :type="showAdminToken ? 'text' : 'password'"
                :append-inner-icon="showAdminToken ? 'mdi-eye' : 'mdi-eye-off'"
                @click:append-inner="showAdminToken = !showAdminToken"
              />
              <v-text-field
                v-model="form.apiKey"
                placeholder="API key"
                variant="outlined"
                density="compact"
                :append-inner-icon="showApiKey ? 'mdi-eye' : 'mdi-eye-off'"
                :type="showApiKey ? 'text' : 'password'"
                @click:append-inner="showApiKey = !showApiKey"
              />
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
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-float')" class="settings-card">
            <v-card-title>Opening cash float</v-card-title>
            <v-card-text class="field-grid">
              <v-text-field v-model.number="form.defaultOpeningFloatUsd" placeholder="Opening float (USD)" type="number" min="0" step="0.01" variant="outlined" density="compact" hide-details />
              <v-text-field v-model.number="form.defaultOpeningFloatLbp" placeholder="Opening float (LBP)" type="number" min="0" step="1" variant="outlined" density="compact" hide-details />
              <v-alert v-if="openingFloatNegative" type="warning" variant="tonal" density="compact" class="field-full">
                Opening float cannot be negative.
              </v-alert>
              <v-btn color="primary" variant="tonal" :disabled="openingFloatNegative" class="field-full" @click="openingFloatDialog = true">Save opening float</v-btn>
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
              <v-table density="compact" class="mb-4">
                <thead>
                  <tr>
                    <th>Code</th>
                    <th>Name</th>
                    <th>Symbol</th>
                    <th>Units per USD</th>
                    <th>Cash rounding unit</th>
                    <th>Active</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="currency in currencies" :key="currency.code">
                    <td>{{ currency.code }}</td>
                    <td>{{ currency.name }}</td>
                    <td>{{ currency.symbol }}</td>
                    <td>{{ currency.unitsPerUsd }}</td>
                    <td>{{ currency.cashRoundingUnit }}</td>
                    <td>
                      <v-switch
                        :model-value="currency.isActive"
                        :disabled="currency.isDefault"
                        hide-details
                        density="compact"
                        color="primary"
                        @update:model-value="toggleCurrencyActive(currency)"
                      />
                    </td>
                    <td>
                      <v-btn
                        v-if="!currency.isDefault"
                        icon="mdi-pencil"
                        variant="text"
                        size="small"
                        @click="editCurrency(currency)"
                      />
                    </td>
                  </tr>
                </tbody>
              </v-table>
              <div class="text-subtitle-2 mb-2">
                {{ editingCurrencyCode ? `Edit ${editingCurrencyCode}` : "Add a currency" }}
              </div>
              <div class="d-flex flex-wrap ga-2 align-start">
                <v-text-field
                  v-if="!editingCurrencyCode"
                  v-model="currencyForm.code"
                  placeholder="Code (e.g. EUR)"
                  variant="outlined"
                  density="compact"
                  style="max-width: 140px"
                />
                <v-text-field v-model="currencyForm.name" placeholder="Name" variant="outlined" density="compact" style="max-width: 180px" />
                <v-text-field v-model="currencyForm.symbol" placeholder="Symbol" variant="outlined" density="compact" style="max-width: 100px" />
                <v-text-field
                  v-model.number="currencyForm.unitsPerUsd"
                  placeholder="Units per USD"
                  type="number"
                  min="0"
                  step="0.0001"
                  variant="outlined"
                  density="compact"
                  style="max-width: 160px"
                />
                <v-text-field
                  v-model.number="currencyForm.cashRoundingUnit"
                  placeholder="Cash rounding unit"
                  type="number"
                  min="1"
                  step="1"
                  variant="outlined"
                  density="compact"
                  style="max-width: 160px"
                />
                <v-btn color="primary" :loading="currencySaving" @click="saveCurrency">
                  {{ editingCurrencyCode ? "Save" : "Add" }}
                </v-btn>
                <v-btn v-if="editingCurrencyCode" variant="text" @click="resetCurrencyForm">Cancel</v-btn>
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-payment-methods')" class="settings-card">
            <v-card-title>Payment methods</v-card-title>
            <v-card-text>
              <v-table density="compact" class="mb-4">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Currency</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="method in paymentMethods" :key="method.id">
                    <td>{{ method.name }}</td>
                    <td>{{ method.currencyCode }}</td>
                  </tr>
                </tbody>
              </v-table>
              <div class="text-subtitle-2 mb-2">Add a payment method</div>
              <div class="d-flex flex-wrap ga-2 align-start">
                <v-text-field
                  v-model="paymentMethodForm.name"
                  placeholder="Name (e.g. Euro Cash)"
                  variant="outlined"
                  density="compact"
                  style="max-width: 220px"
                />
                <v-select
                  v-model="paymentMethodForm.currencyCode"
                  :items="currencies.filter((c) => c.isActive).map((c) => ({ title: `${c.name} (${c.code})`, value: c.code }))"
                  placeholder="Currency"
                  variant="outlined"
                  density="compact"
                  style="max-width: 220px"
                />
                <v-btn color="primary" :loading="paymentMethodSaving" @click="savePaymentMethod">Add</v-btn>
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-price-lists')" class="settings-card">
            <v-card-title>Price lists</v-card-title>
            <v-card-text>
              <p class="text-caption text-medium-emphasis mb-3">
                A price list overrides a product code's price for every color/size that shares it
                (e.g. "Black Friday" -> lady pants $11). Enable a list to apply it at checkout; a
                product code can only be overridden by one active price list at a time.
              </p>
              <v-table density="compact" class="mb-4">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Active</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="list in priceLists" :key="list.id">
                    <td>{{ list.name }}</td>
                    <td>
                      <v-switch
                        :model-value="list.isActive"
                        hide-details
                        density="compact"
                        color="primary"
                        @update:model-value="togglePriceListActive(list)"
                      />
                    </td>
                    <td class="d-flex ga-1">
                      <v-btn icon="mdi-format-list-bulleted-square" variant="text" size="small" title="Manage items" @click="openManageList('price', list)" />
                      <v-btn icon="mdi-pencil" variant="text" size="small" @click="editPriceList(list)" />
                    </td>
                  </tr>
                </tbody>
              </v-table>
              <div class="text-subtitle-2 mb-2">
                {{ editingPriceListId ? "Edit price list" : "Add a price list" }}
              </div>
              <div class="d-flex flex-wrap ga-2 align-start">
                <v-text-field v-model="priceListForm.name" placeholder="Name (e.g. Black Friday)" variant="outlined" density="compact" style="max-width: 240px" />
                <v-btn color="primary" :loading="priceListSaving" @click="savePriceList">
                  {{ editingPriceListId ? "Save" : "Add" }}
                </v-btn>
                <v-btn v-if="editingPriceListId" variant="text" @click="resetPriceListForm">Cancel</v-btn>
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('money-discount-lists')" class="settings-card">
            <v-card-title>Discount lists</v-card-title>
            <v-card-text>
              <p class="text-caption text-medium-emphasis mb-3">
                A discount list overrides a product code's discount %, independently of any price
                list. A product code can only be overridden by one active discount list at a time.
              </p>
              <v-table density="compact" class="mb-4">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Active</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="list in discountLists" :key="list.id">
                    <td>{{ list.name }}</td>
                    <td>
                      <v-switch
                        :model-value="list.isActive"
                        hide-details
                        density="compact"
                        color="primary"
                        @update:model-value="toggleDiscountListActive(list)"
                      />
                    </td>
                    <td class="d-flex ga-1">
                      <v-btn icon="mdi-format-list-bulleted-square" variant="text" size="small" title="Manage items" @click="openManageList('discount', list)" />
                      <v-btn icon="mdi-pencil" variant="text" size="small" @click="editDiscountList(list)" />
                    </td>
                  </tr>
                </tbody>
              </v-table>
              <div class="text-subtitle-2 mb-2">
                {{ editingDiscountListId ? "Edit discount list" : "Add a discount list" }}
              </div>
              <div class="d-flex flex-wrap ga-2 align-start">
                <v-text-field v-model="discountListForm.name" placeholder="Name" variant="outlined" density="compact" style="max-width: 240px" />
                <v-btn color="primary" :loading="discountListSaving" @click="saveDiscountList">
                  {{ editingDiscountListId ? "Save" : "Add" }}
                </v-btn>
                <v-btn v-if="editingDiscountListId" variant="text" @click="resetDiscountListForm">Cancel</v-btn>
              </div>
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
        <v-card v-show="panelMatches('coupons-types')" class="settings-card" max-width="720">
          <v-card-title>Coupon types</v-card-title>
          <v-card-text>
            <div class="d-flex ga-2 align-start">
              <v-text-field v-model="couponTypeForm.name" placeholder="Type name" variant="outlined" density="compact" />
              <v-text-field v-model.number="couponTypeForm.value" placeholder="Value" type="number" min="0.01" step="0.01" variant="outlined" density="compact" />
              <v-text-field v-model="couponTypeForm.currency" placeholder="Currency" maxlength="3" variant="outlined" density="compact" style="max-width: 110px" />
              <v-btn color="primary" :loading="couponTypeSaving" @click="saveCouponType">{{ editingCouponTypeId ? "Save" : "Add" }}</v-btn>
              <v-btn v-if="editingCouponTypeId" variant="text" @click="cancelCouponTypeEdit">Cancel</v-btn>
            </div>
            <div class="d-flex align-center ga-4 mt-n3 mb-2">
              <v-checkbox v-model="couponTypeForm.referenceOnly" density="compact" hide-details label="Reference-only" />
              <v-checkbox v-model="couponTypeForm.isActive" density="compact" hide-details label="Active" />
            </div>
            <div class="d-flex ga-2 align-start mb-2">
              <v-text-field v-model="couponCodeForm" placeholder="New coupon code" variant="outlined" density="compact" />
              <v-select v-model="couponIssueTypeId" :items="couponTypes" item-title="name" item-value="id" placeholder="Type" variant="outlined" density="compact" />
              <v-btn color="secondary" :loading="couponIssuing" @click="issueCoupon">Issue</v-btn>
            </div>
            <v-list density="compact">
              <v-list-item v-for="couponType in couponTypes" :key="couponType.id" :title="couponType.name" :subtitle="`${(Number(couponType.valueCents) / 100).toFixed(2)} ${couponType.currencyCode}${couponType.referenceOnly ? ' · Reference-only' : ''}`">
                <template #append>
                  <v-btn icon="mdi-pencil-outline" variant="text" density="compact" title="Edit coupon type" @click="editCouponType(couponType)" />
                  <v-btn icon="mdi-delete-outline" variant="text" density="compact" title="Delete coupon type" @click="removeCouponType(couponType)" />
                </template>
              </v-list-item>
              <v-list-item v-if="!couponTypes.length" title="No coupon types configured" />
            </v-list>
            <div class="text-subtitle-2 mt-4 mb-1">Issued coupons</div>
            <v-list density="compact">
              <v-list-item v-for="coupon in coupons" :key="coupon.code" :title="coupon.code" :subtitle="`${(Number(coupon.valueCents) / 100).toFixed(2)} · ${coupon.status}`">
                <template #append><v-btn icon="mdi-delete-outline" variant="text" density="compact" title="Delete coupon" @click="removeCoupon(coupon)" /></template>
              </v-list-item>
              <v-list-item v-if="!coupons.length" title="No coupons issued" />
            </v-list>
          </v-card-text>
        </v-card>
      </v-window-item>

      <v-window-item value="admin">
        <div class="settings-grid">
          <v-card v-show="panelMatches('admin-custom-pricing')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-sale-outline" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Custom item pricing</div>
                <div class="text-caption text-medium-emphasis">
                  Choose which item-level price and discount controls cashiers can use.
                </div>
              </div>
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.customItemPricesEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Enable custom item prices"
                @update:model-value="setCustomPricesEnabled"
              />
              <v-switch
                :model-value="form.perUnitItemPricesEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Enable per-unit price and discount editing"
                @update:model-value="setPerUnitPricesEnabled"
              />
              <v-switch
                :model-value="form.customItemDiscountsEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Enable custom product discounts"
                @update:model-value="setCustomDiscountsEnabled"
              />
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('admin-price-change-mode')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-tag-multiple-outline" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Price change mode</div>
                <div class="text-caption text-medium-emphasis">
                  Choose how a manual price edit relates to the item's discount.
                </div>
              </div>
            </v-card-title>
            <v-card-text>
              <v-switch
                :model-value="form.priceChangeManualOverrideEnabled"
                :loading="pricingSettingsSaving"
                color="primary"
                hide-details
                label="Treat price changes as manual overrides"
                @update:model-value="setPriceChangeManualOverrideEnabled"
              />
              <div class="text-caption text-medium-emphasis mt-2">
                {{
                  form.priceChangeManualOverrideEnabled
                    ? "Manual override: price and discount are edited independently, as separate fields."
                    : "Discount (default): price and discount are the same lever — changing either one recalculates the other from the item's original price."
                }}
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('admin-invoice-keyboard')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-keyboard" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Invoice keyboard mode</div>
                <div class="text-caption text-medium-emphasis">
                  Edit qty/discount/price on the invoice by clicking a value and typing digits,
                  instead of opening the number pad.
                </div>
              </div>
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
              <div class="text-caption text-medium-emphasis mt-2">
                With keyboard mode on, clicking qty, discount, or price focuses it - type a
                number to set it, then use the arrow keys to move to the next field. Click the
                barcode field to return focus there.
              </div>
            </v-card-text>
          </v-card>

          <v-card v-show="panelMatches('admin-page-access')" class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-lock-outline" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Page access</div>
                <div class="text-caption text-medium-emphasis">
                  Choose which pages non-admin users can open. Admins always see every page.
                </div>
              </div>
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
                <v-avatar color="primary" variant="tonal" size="36">
                  <v-icon icon="mdi-lock-reset" size="20" />
                </v-avatar>
                <div>
                  <div class="text-subtitle-1 font-weight-bold">Change password</div>
                  <div class="text-caption text-medium-emphasis">Keep your account secure.</div>
                </div>
              </v-card-title>
              <v-card-text>
                <v-text-field
                  v-model="passwordForm.current"
                  placeholder="Current Password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                  :append-inner-icon="showPw ? 'mdi-eye' : 'mdi-eye-off'"
                  @click:append-inner="showPw = !showPw"
                />
                <v-text-field
                  v-model="passwordForm.newPass"
                  placeholder="New Password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                />
                <v-text-field
                  v-model="passwordForm.confirm"
                  placeholder="Confirm New Password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                  :rules="[(v) => v === passwordForm.newPass || 'Passwords do not match']"
                />
              </v-card-text>
              <v-card-actions>
                <v-spacer />
                <v-btn color="primary" variant="flat" type="submit" :loading="changingPassword">
                  <v-icon start icon="mdi-lock-reset" /> Change Password
                </v-btn>
              </v-card-actions>
            </v-card>
          </v-form>

          <v-card v-show="panelMatches('admin-users')" class="settings-card settings-card--wide">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-account-group-outline" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Users</div>
                <div class="text-caption text-medium-emphasis">
                  Manage who can access the point of sale.
                </div>
              </div>
              <v-spacer />
              <v-btn color="primary" size="small" @click="openUserDialog()">
                <v-icon start icon="mdi-plus" /> Add user
              </v-btn>
            </v-card-title>
            <v-data-table
              :headers="userHeaders"
              :items="users"
              items-per-page="-1"
              hide-default-footer
              density="comfortable"
              hover
            >
              <template #item.active="{ item }">
                <v-chip :color="item.active ? 'green' : 'red'" size="small" variant="tonal">
                  {{ item.active ? "Active" : "Disabled" }}
                </v-chip>
              </template>
              <template #item.actions="{ item }">
                <v-btn icon size="small" variant="text" @click="openUserDialog(item)">
                  <v-icon icon="mdi-pencil-outline" size="18" />
                </v-btn>
                <v-btn icon size="small" variant="text" @click="deleteUserConfirm(item)">
                  <v-icon icon="mdi-delete-outline" size="18" color="red" />
                </v-btn>
              </template>
            </v-data-table>
          </v-card>

          <v-card v-show="panelMatches('admin-salespersons')" class="settings-card settings-card--wide">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-account-tie" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Salespersons</div>
                <div class="text-caption text-medium-emphasis">Manage the people assigned to sales.</div>
              </div>
              <v-spacer />
              <v-btn color="primary" size="small" @click="openSalespersonDialog()">
                <v-icon start icon="mdi-plus" /> Add salesperson
              </v-btn>
            </v-card-title>
            <v-data-table
              :headers="salespersonHeaders"
              :items="salespersons"
              items-per-page="-1"
              hide-default-footer
              density="comfortable"
              hover
            >
              <template #item.actions="{ item }">
                <v-btn icon size="small" variant="text" @click="openSalespersonDialog(item)">
                  <v-icon icon="mdi-pencil-outline" size="18" />
                </v-btn>
                <v-btn icon size="small" variant="text" @click="deleteSalespersonConfirm(item)">
                  <v-icon icon="mdi-delete-outline" size="18" color="red" />
                </v-btn>
              </template>
            </v-data-table>
          </v-card>
        </div>
      </v-window-item>
    </v-window>

    <v-dialog v-model="userDialog" max-width="400">
      <v-card>
        <v-card-title>{{ editingUser ? "Edit User" : "New User" }}</v-card-title>
        <v-card-text>
          <v-text-field
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
            :placeholder="editingUser ? 'Leave blank to keep current' : 'Password'"
          />
          <v-select
            v-model="userForm.roleId"
            :items="roles"
            item-title="name"
            item-value="id"
            placeholder="Role"
            variant="outlined"
            density="compact"
            class="mb-2"
          />
          <v-text-field v-model="userForm.attendanceUserId" placeholder="Akuvox UserID (leave blank for visitors)" variant="outlined" density="compact" />
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="userDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn
            color="primary"
            variant="flat"
            :disabled="!userForm.username"
            :loading="savingUser"
            @click="saveUser"
            >Save</v-btn
          >
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
        <v-card-title>{{ editingSalesperson ? "Edit Salesperson" : "New Salesperson" }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="salespersonName" placeholder="Full name" variant="outlined" density="compact" autofocus />
          <v-text-field v-model="salespersonAttendanceUserId" placeholder="Akuvox attendance ID" variant="outlined" density="compact" />
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="salespersonDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn color="primary" variant="flat" :disabled="!salespersonName.trim()" :loading="savingSalesperson" @click="saveSalesperson">Save</v-btn>
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
          <v-table density="compact" class="mb-4">
            <thead>
              <tr>
                <th>Product code</th>
                <th>{{ manageListType === "price" ? "Price" : "Discount" }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in manageListItems" :key="item.id">
                <td>{{ item.productCode }}</td>
                <td>{{ manageListType === "price" ? `$${Number(item.price).toFixed(2)}` : `${item.discount}%` }}</td>
                <td>
                  <v-btn icon="mdi-delete-outline" variant="text" size="small" @click="removeManageListItem(item)" />
                </td>
              </tr>
              <tr v-if="!manageListItems.length">
                <td colspan="3" class="text-medium-emphasis">No items yet.</td>
              </tr>
            </tbody>
          </v-table>
          <div class="d-flex flex-wrap ga-2 align-start">
            <v-text-field v-model="manageItemCode" placeholder="Product code" variant="outlined" density="compact" style="max-width: 200px" />
            <ProductSearchDialog activator-text="Pick product" activator-icon="mdi-magnify" @select="pickManageItemProduct" />
            <v-text-field
              v-model.number="manageItemValue"
              :placeholder="manageListType === 'price' ? 'Price ($)' : 'Discount (%)'"
              type="number"
              min="0"
              :step="manageListType === 'price' ? 0.01 : 1"
              variant="outlined"
              density="compact"
              style="max-width: 160px"
            />
            <v-btn color="primary" :loading="manageItemSaving" @click="addManageListItem">Add / update</v-btn>
          </div>
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
import { ref, reactive, computed, onMounted, watch } from "vue";
import {
  GetAppSettings,
  GetDatabaseStatus,
  ChooseDatabase,
  UpdateSettings,
  UpdateAttendanceConfig,
  UpdateAttendanceApproverPassword,
  SetAttendanceDuplicateInterval,
  SetExchangeWindowHours,
  SetCustomItemDiscountsEnabled,
  SetCustomItemPricesEnabled,
  SetPerUnitItemPricesEnabled,
  SetPriceChangeManualOverrideEnabled,
  SetInvoiceKeyboardModeEnabled,
  SetPageUnlocked,
  UpdateDefaultOpeningFloat,
  SetAutoCloseStaleShiftsEnabled,
  SetAttendanceCashierHistoryEnabled,
  UpdateUserAttendanceID,
  UpdateSalespersonAttendanceID,
  UpdateReceiptSettings,
  ChangePassword,
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
  CreateCurrency,
  UpdateCurrency,
  SetCurrencyActive,
  ListPaymentMethods,
  CreateCashboxAccount,
  SetTheme,
  ListPriceLists,
  CreatePriceList,
  UpdatePriceList,
  SetPriceListActive,
  ListPriceListItems,
  SetPriceListItem,
  RemovePriceListItem,
  ListDiscountLists,
  CreateDiscountList,
  UpdateDiscountList,
  SetDiscountListActive,
  ListDiscountListItems,
  SetDiscountListItem,
  RemoveDiscountListItem,
} from "../../wailsjs/go/main/App";
import { WindowReloadApp } from "../../wailsjs/runtime/runtime";
import { useAuthStore } from "@/stores/auth";
import { usePageUnlocks, lockablePages } from "@/composables/usePageUnlocks";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import {
  animationsEnabled,
  setAnimationsEnabled,
  calculatorTotalCurrency,
  setCalculatorTotalCurrency,
} from "@/lib/display";
import { useTheme } from "vuetify";

const auth = useAuthStore();
const theme = useTheme();
const tab = ref("general");

// Static index of every settings "panel" (its tab + searchable text) used by
// the search box below. Keep in sync with the v-show="panelMatches(...)" on
// each panel in the template.
const panelMeta: Record<string, { tab: string; title: string; keywords: string }> = {
  "general-appearance": { tab: "general", title: "Appearance", keywords: "dark theme animations mode display" },
  "general-branch": { tab: "general", title: "Branch", keywords: "branch name code sales inventory" },
  "general-database": { tab: "general", title: "Database", keywords: "database path choose switch" },
  "general-remote": { tab: "general", title: "Remote admin & API", keywords: "remote admin api listen address token key" },
  "attendance-machine": { tab: "attendance", title: "Attendance machine", keywords: "akuvox ip username password manager duplicate interval enable cashier history auto close stale shifts" },
  "printing-printer": { tab: "printing", title: "Printer & customer screen", keywords: "receipt printer size width height print customer screen com port" },
  "printing-receipt": { tab: "printing", title: "Receipt settings", keywords: "market name phone description branch phone register return policy social platforms instagram facebook website font body size title size cut paper sample" },
  "money-general": { tab: "money", title: "Money settings", keywords: "vat tax rate exchange rate overpayment rounding mode currency calculator total currency" },
  "money-float": { tab: "money", title: "Opening cash float", keywords: "opening cash float usd lbp drawer shift" },
  "money-exchange": { tab: "money", title: "Exchange settings", keywords: "exchange window days hours return" },
  "money-currencies": { tab: "money", title: "Currencies & rates", keywords: "currency code name symbol units per usd cash rounding unit active add currency" },
  "money-payment-methods": { tab: "money", title: "Payment methods", keywords: "payment method cashbox account currency" },
  "money-price-lists": { tab: "money", title: "Price lists", keywords: "price list override product code black friday active" },
  "money-discount-lists": { tab: "money", title: "Discount lists", keywords: "discount list override product code percent active" },
  "coupons-types": { tab: "coupons", title: "Coupon types", keywords: "coupon type value currency reference only active issue code" },
  "admin-custom-pricing": { tab: "admin", title: "Custom item pricing", keywords: "custom item price discount per unit editing" },
  "admin-price-change-mode": { tab: "admin", title: "Price change mode", keywords: "price change manual override discount lever" },
  "admin-invoice-keyboard": { tab: "admin", title: "Invoice keyboard mode", keywords: "invoice keyboard qty discount price number pad" },
  "admin-page-access": { tab: "admin", title: "Page access", keywords: "page access lock unlock non-admin clients inventory transfers attendance salespersons" },
  "admin-change-password": { tab: "admin", title: "Change password", keywords: "change password current new confirm account security" },
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
    if (panelMatches(key)) set.add(panelMeta[key].tab);
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
  printerID: "",
  printerSize: "80mm",
  receiptPrinter: "",
  screenPort: "",
  receiptWidth: 80,
  receiptHeight: 297,
  taxRate: 0,
  roundingMode: "half_up",
  roundingCurrency: "LBP",
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
const currencyForm = reactive({ code: "", name: "", symbol: "", unitsPerUsd: 1, cashRoundingUnit: 1 });
const editingCurrencyCode = ref<string | null>(null);
const currencySaving = ref(false);
const paymentMethodForm = reactive({ name: "", currencyCode: "" });
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

function editCurrency(currency: any) {
  editingCurrencyCode.value = currency.code;
  Object.assign(currencyForm, {
    code: currency.code,
    name: currency.name,
    symbol: currency.symbol,
    unitsPerUsd: currency.unitsPerUsd,
    cashRoundingUnit: currency.cashRoundingUnit,
  });
}

function resetCurrencyForm() {
  editingCurrencyCode.value = null;
  Object.assign(currencyForm, { code: "", name: "", symbol: "", unitsPerUsd: 1, cashRoundingUnit: 1 });
}

async function saveCurrency() {
  const unitsPerUsd = Number(currencyForm.unitsPerUsd);
  const cashRoundingUnit = Math.round(Number(currencyForm.cashRoundingUnit));
  if (!currencyForm.name.trim() || !currencyForm.symbol.trim() || !Number.isFinite(unitsPerUsd) || unitsPerUsd <= 0) {
    notify("Name, symbol, and a positive rate are required", "red");
    return;
  }
  if (!Number.isFinite(cashRoundingUnit) || cashRoundingUnit <= 0) {
    notify("Cash rounding unit must be a positive whole number", "red");
    return;
  }
  currencySaving.value = true;
  try {
    if (editingCurrencyCode.value) {
      await UpdateCurrency(editingCurrencyCode.value, currencyForm.name.trim(), currencyForm.symbol.trim(), unitsPerUsd, cashRoundingUnit);
    } else {
      if (!currencyForm.code.trim()) {
        notify("Currency code is required", "red");
        currencySaving.value = false;
        return;
      }
      await CreateCurrency(currencyForm.code.trim(), currencyForm.name.trim(), currencyForm.symbol.trim(), unitsPerUsd, cashRoundingUnit);
    }
    await loadCurrencies();
    resetCurrencyForm();
    notify("Currency saved");
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to save currency", "red");
  } finally {
    currencySaving.value = false;
  }
}

async function toggleCurrencyActive(currency: any) {
  try {
    await SetCurrencyActive(currency.code, !currency.isActive);
    await loadCurrencies();
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to update currency", "red");
  }
}

async function savePaymentMethod() {
  if (!paymentMethodForm.name.trim() || !paymentMethodForm.currencyCode) {
    notify("Name and currency are required", "red");
    return;
  }
  paymentMethodSaving.value = true;
  try {
    await CreateCashboxAccount(paymentMethodForm.name.trim(), paymentMethodForm.currencyCode);
    await loadPaymentMethodsList();
    paymentMethodForm.name = "";
    notify("Payment method added");
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to add payment method", "red");
  } finally {
    paymentMethodSaving.value = false;
  }
}
const priceLists = ref<any[]>([]);
const priceListForm = reactive({ name: "" });
const priceListSaving = ref(false);
const editingPriceListId = ref<number | null>(null);

const discountLists = ref<any[]>([]);
const discountListForm = reactive({ name: "" });
const discountListSaving = ref(false);
const editingDiscountListId = ref<number | null>(null);

const manageListDialog = ref(false);
const manageListType = ref<"price" | "discount">("price");
const manageListId = ref<number | null>(null);
const manageListName = ref("");
const manageListItems = ref<any[]>([]);
const manageItemCode = ref("");
const manageItemValue = ref<number | null>(null);
const manageItemSaving = ref(false);

async function loadPriceLists() {
  try {
    priceLists.value = (await ListPriceLists()) as any[];
  } catch {
    priceLists.value = [];
  }
}

function editPriceList(list: any) {
  editingPriceListId.value = list.id;
  priceListForm.name = list.name;
}

function resetPriceListForm() {
  editingPriceListId.value = null;
  priceListForm.name = "";
}

async function savePriceList() {
  const name = priceListForm.name.trim();
  if (!name) {
    notify("Name is required", "red");
    return;
  }
  priceListSaving.value = true;
  try {
    if (editingPriceListId.value) await UpdatePriceList(editingPriceListId.value, name);
    else await CreatePriceList(name);
    await loadPriceLists();
    resetPriceListForm();
    notify(editingPriceListId.value ? "Price list updated" : "Price list created");
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to save price list", "red");
  } finally {
    priceListSaving.value = false;
  }
}

async function togglePriceListActive(list: any) {
  try {
    await SetPriceListActive(list.id, !list.isActive);
    await loadPriceLists();
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to update price list", "red");
  }
}

async function loadDiscountLists() {
  try {
    discountLists.value = (await ListDiscountLists()) as any[];
  } catch {
    discountLists.value = [];
  }
}

function editDiscountList(list: any) {
  editingDiscountListId.value = list.id;
  discountListForm.name = list.name;
}

function resetDiscountListForm() {
  editingDiscountListId.value = null;
  discountListForm.name = "";
}

async function saveDiscountList() {
  const name = discountListForm.name.trim();
  if (!name) {
    notify("Name is required", "red");
    return;
  }
  discountListSaving.value = true;
  try {
    if (editingDiscountListId.value) await UpdateDiscountList(editingDiscountListId.value, name);
    else await CreateDiscountList(name);
    await loadDiscountLists();
    resetDiscountListForm();
    notify(editingDiscountListId.value ? "Discount list updated" : "Discount list created");
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to save discount list", "red");
  } finally {
    discountListSaving.value = false;
  }
}

async function toggleDiscountListActive(list: any) {
  try {
    await SetDiscountListActive(list.id, !list.isActive);
    await loadDiscountLists();
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to update discount list", "red");
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
  manageItemCode.value = "";
  manageItemValue.value = null;
  manageListDialog.value = true;
  await loadManageListItems();
}

function pickManageItemProduct(product: any) {
  manageItemCode.value = product.code;
}

async function addManageListItem() {
  if (!manageListId.value || !manageItemCode.value.trim() || manageItemValue.value === null) {
    notify("Product code and a value are required", "red");
    return;
  }
  manageItemSaving.value = true;
  try {
    if (manageListType.value === "price") {
      await SetPriceListItem(manageListId.value, manageItemCode.value.trim(), Number(manageItemValue.value));
    } else {
      await SetDiscountListItem(manageListId.value, manageItemCode.value.trim(), Math.round(Number(manageItemValue.value)));
    }
    manageItemCode.value = "";
    manageItemValue.value = null;
    await loadManageListItems();
    await (manageListType.value === "price" ? loadPriceLists() : loadDiscountLists());
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to save item", "red");
  } finally {
    manageItemSaving.value = false;
  }
}

async function removeManageListItem(item: any) {
  try {
    if (manageListType.value === "price") await RemovePriceListItem(item.id);
    else await RemoveDiscountListItem(item.id);
    await loadManageListItems();
    await (manageListType.value === "price" ? loadPriceLists() : loadDiscountLists());
  } catch (error) {
    notify(error instanceof Error ? error.message : "Failed to remove item", "red");
  }
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
const saving = ref(false);
const couponTypes = ref<any[]>([]);
const coupons = ref<any[]>([]);
const couponTypeForm = reactive({ name: "", value: 50, currency: "USD", referenceOnly: false, isActive: true });
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
const userForm = reactive({ username: "", password: "", roleId: 1, attendanceUserId: "" });
const showUpw = ref(false);
const savingUser = ref(false);

const deleteDialog = ref(false);
const deleteTarget = ref<any>(null);

const salespersons = ref<any[]>([]);
const salespersonHeaders = [
  { title: "Name", key: "fullName" },
  { title: "Attendance ID", key: "attendanceUserId" },
  { title: "", key: "actions", sortable: false, align: "end" as const },
];
const salespersonDialog = ref(false);
const editingSalesperson = ref<any>(null);
const salespersonName = ref("");
const salespersonAttendanceUserId = ref("");
const savingSalesperson = ref(false);
const deleteSalespersonDialog = ref(false);
const deleteSalespersonTarget = ref<any>(null);

const snackbar = reactive({ show: false, text: "", color: "success" });

const adminBridge = () => (window as any).go?.main?.App;

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
}

function cancelCouponTypeEdit() {
  editingCouponTypeId.value = null;
  couponTypeForm.name = "";
  couponTypeForm.value = 50;
  couponTypeForm.currency = "USD";
  couponTypeForm.referenceOnly = false;
  couponTypeForm.isActive = true;
}

async function saveCouponType() {
  if (!couponTypeForm.name.trim() || Number(couponTypeForm.value) <= 0) return;
  couponTypeSaving.value = true;
  try {
    const valueCents = Math.round(Number(couponTypeForm.value) * 100);
    if (editingCouponTypeId.value) {
      await UpdateCouponType(editingCouponTypeId.value, couponTypeForm.name.trim(), couponTypeForm.currency, valueCents, couponTypeForm.isActive, couponTypeForm.referenceOnly);
    } else {
      await CreateCouponType(couponTypeForm.name.trim(), couponTypeForm.currency, valueCents, couponTypeForm.referenceOnly);
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
        printerID: s.printerID ?? "",
        printerSize: s.printerSize ?? "80mm",
        receiptPrinter: s.receiptPrinter ?? "",
        screenPort: s.screenPort ?? "",
        receiptWidth: Number(s.receiptWidth ?? 80),
        receiptHeight: Number(s.receiptHeight ?? 297),
        taxRate: Number(s.taxRate ?? 0),
        roundingMode: s.roundingMode ?? "half_up",
        roundingCurrency: s.roundingCurrency ?? "LBP",
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

onMounted(async () => {
  await loadDatabaseStatus();
  await loadCouponTypes();
  await loadSettings();
  await loadCurrencies();
  await loadPaymentMethodsList();
  await loadPriceLists();
  await loadDiscountLists();
  try {
    serialPorts.value = await ListSerialPorts();
  } catch {}
  try {
    printers.value = await ListPrinters();
  } catch {}
  try {
    inventories.value = await ListInventories();
  } catch {}
  await loadUsers();
  await loadSalespersons();
  try {
    roles.value = await ListRoles();
  } catch {}
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
  if (!salespersonName.value.trim()) return;
  savingSalesperson.value = true;
  try {
    if (editingSalesperson.value) {
      await UpdateSalesperson(editingSalesperson.value.id, salespersonName.value);
      await UpdateSalespersonAttendanceID(editingSalesperson.value.id, salespersonAttendanceUserId.value);
    } else {
      const created = await CreateSalesperson(salespersonName.value);
      await UpdateSalespersonAttendanceID(Number(created.id), salespersonAttendanceUserId.value);
    }
    salespersonDialog.value = false;
    notify(editingSalesperson.value ? "Salesperson updated" : "Salesperson created");
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
    await ChangePassword(auth.currentUser!.username, passwordForm.current, passwordForm.newPass);
    notify("Password changed");
    passwordForm.current = "";
    passwordForm.newPass = "";
    passwordForm.confirm = "";
  } catch {
    notify("Failed to change password", "red");
  }
  changingPassword.value = false;
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
    if (editingUser.value) {
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
.settings-page {
  box-sizing: border-box;
  height: 100%;
  overflow-y: auto;
  padding: 28px 32px 40px;
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
}

.settings-tabs :deep(.v-tab) {
  min-width: 0;
  padding-inline: 18px;
  text-transform: none;
  font-weight: 600;
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

.settings-card :deep(.v-card-title .text-subtitle-1) {
  font-size: inherit !important;
  line-height: inherit;
}

.settings-card :deep(.v-card-title .text-caption) {
  font-weight: 400;
}

.settings-card :deep(.v-card-text) {
  padding: 12px 18px 16px;
}

.settings-card :deep(.v-table) {
  background: transparent;
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

  .settings-page {
    padding: 16px;
  }
}
</style>
