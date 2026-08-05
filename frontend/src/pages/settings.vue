<template>
  <div class="settings-page">
    <!-- <div class="settings-heading"> -->
    <!--   <div> -->
    <!--     <div class="text-overline text-primary font-weight-bold">Workspace configuration</div> -->
    <!--     <h1 class="text-h4 font-weight-bold mb-1">Settings</h1> -->
    <!--     <p class="text-body-2 text-medium-emphasis mb-0"> -->
    <!--       Configure receipts, customer displays, security, and user access. -->
    <!--     </p> -->
    <!--   </div> -->
    <!--   <v-icon icon="mdi-cog-outline" size="48" class="settings-heading-icon" /> -->
    <!-- </div> -->

    <!-- <v-alert v-if="settingsLoadError" type="warning" variant="tonal" class="mb-4"> -->
    <!--   Settings could not be loaded from the database. The form is using safe defaults. -->
    <!-- </v-alert> -->

    <v-tabs v-model="tab" class="settings-tabs mb-5" color="primary" grow>
      <v-tab value="general" prepend-icon="mdi-printer-wireless">Printing &amp; Screen</v-tab>
      <v-tab value="receipt" prepend-icon="mdi-printer-outline">Receipt</v-tab>
      <v-tab value="coupons" prepend-icon="mdi-sale-outline">Coupons</v-tab>
      <v-tab value="admin" prepend-icon="mdi-shield-account-outline">Passwords &amp; Users</v-tab>
    </v-tabs>
    <v-progress-linear v-if="settingsLoading" indeterminate color="primary" class="mb-4" />

    <v-window v-model="tab">
      <v-window-item value="general">
        <div class="settings-grid">
          <v-card class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-printer-outline" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Printing preferences</div>
                <div class="text-caption text-medium-emphasis">
                  Set up your receipt printer and layout.
                </div>
              </div>
            </v-card-title>
            <v-card-text>
              <v-text-field
                v-model="form.branchName"
                label="Branch Name"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-text-field
                v-model="form.branchCode"
                label="Branch Code"
                hint="Used in invoice numbers, for example CN."
                maxlength="8"
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
                @update:model-value="form.branchCode = form.branchCode.toUpperCase()"
              />
              <v-text-field
                v-model="form.apiKey"
                label="API Key"
                variant="outlined"
                density="compact"
                class="mb-2"
                :append-inner-icon="showApiKey ? 'mdi-eye' : 'mdi-eye-off'"
                :type="showApiKey ? 'text' : 'password'"
                @click:append-inner="showApiKey = !showApiKey"
              />
              <v-select
                v-model="form.receiptPrinter"
                :items="printers"
                label="Receipt Printer"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-select
                v-model="form.printerSize"
                :items="['58mm', '80mm']"
                label="Printing Size"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-row density="comfortable">
                <v-col cols="6">
                  <v-text-field
                    v-model.number="form.receiptWidth"
                    label="Width (mm)"
                    type="number"
                    variant="outlined"
                    density="compact"
                  />
                </v-col>
                <v-col cols="6">
                  <v-text-field
                    v-model.number="form.receiptHeight"
                    label="Height (mm)"
                    type="number"
                    variant="outlined"
                    density="compact"
                  />
                </v-col>
              </v-row>
              <v-text-field
                v-model.number="form.taxRate"
                label="VAT rate"
                hint="Use 0.11 for 11%, or 0 to disable VAT."
                persistent-hint
                type="number"
                step="0.01"
                min="0"
                max="1"
                variant="outlined"
                density="compact"
              />
              <v-text-field
                v-model.number="form.exchangeRate"
                label="Exchange rate (LBP per USD)"
                hint="Used to show and accept change in Lebanese pounds."
                persistent-hint
                type="number"
                min="1"
                step="1"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-switch
                v-model="form.allowOverpayment"
                label="Allow overpayment"
                hint="Disabled by default. Enable this to accept more than the invoice total."
                persistent-hint
                color="primary"
                hide-details="auto"
                class="mb-2"
              />
              <v-text-field
                v-model.number="form.maxOverpayment"
                label="Maximum overpayment (USD)"
                hint="Maximum amount above the invoice total. Enable overpayment first."
                persistent-hint
                type="number"
                min="0"
                step="0.01"
                :disabled="!form.allowOverpayment"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-select
                v-model="form.roundingMode"
                :items="roundingModes"
                label="Money rounding"
                hint="Applied to discounts and VAT in cents."
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-select
                v-model.number="form.salesInventoryId"
                :items="inventories"
                item-title="name"
                item-value="id"
                label="Sales inventory"
                hint="Stock is deducted from this inventory."
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-switch v-model="form.receiptEnabled" label="Print receipts after checkout" color="primary" hide-details />
              <v-switch
                :model-value="animationsEnabled"
                label="Animations"
                color="primary"
                hint="Turn off decorative motion for a simpler, faster display."
                persistent-hint
                hide-details
                @update:model-value="(value) => setAnimationsEnabled(value ?? false)"
              />
              <v-divider class="my-4" />
              <div class="text-subtitle-2 font-weight-bold mb-2">Attendance machine</div>
              <v-text-field v-model="form.akuvoxIP" label="Akuvox IP address" placeholder="192.168.1.100" variant="outlined" density="compact" class="mb-2" />
              <v-text-field v-model="form.akuvoxUsername" label="Device username (optional)" variant="outlined" density="compact" class="mb-2" />
              <v-text-field v-model="form.akuvoxPassword" label="Device password (optional)" type="password" variant="outlined" density="compact" class="mb-2" />
              <v-switch v-model="form.attendanceEnabled" label="Enable attendance" color="primary" hide-details />
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
              <v-spacer />
              <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
                <v-icon start icon="mdi-content-save-outline" /> Save changes
              </v-btn>
            </v-card-actions>
          </v-card>

          <v-card class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-monitor-dashboard" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Customer screen</div>
                <div class="text-caption text-medium-emphasis">
                  Choose the display used at checkout.
                </div>
              </div>
            </v-card-title>
            <v-card-text>
              <v-select
                v-model="form.screenPort"
                :items="serialPorts"
                label="Serial Port"
                hint="Leave empty if no customer display is connected."
                persistent-hint
                variant="outlined"
                density="compact"
              />
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
              <v-spacer />
              <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
                <v-icon start icon="mdi-content-save-outline" /> Save changes
              </v-btn>
            </v-card-actions>
          </v-card>
        </div>
      </v-window-item>

      <v-window-item value="receipt">
        <div class="settings-grid settings-grid--receipt">
          <v-card class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36"><v-icon icon="mdi-printer-outline" size="20" /></v-avatar>
              <div><div class="text-subtitle-1 font-weight-bold">Receipt settings</div><div class="text-caption text-medium-emphasis">Customize the printed market identity and footer.</div></div>
            </v-card-title>
            <v-card-text>
              <v-text-field v-model="form.marketName" label="Market name" hint="Printed large at the top, separately from the branch." persistent-hint variant="outlined" density="compact" class="mb-2" />
              <v-text-field v-model="form.marketPhone" label="Phone number" variant="outlined" density="compact" class="mb-2" />
              <v-text-field v-model="form.marketDescription" label="Description / tagline" variant="outlined" density="compact" class="mb-2" />
              <v-text-field v-model="form.branchPhone" label="Branch phone number" hint="Printed below the description." persistent-hint variant="outlined" density="compact" class="mb-2" />
              <v-text-field
                v-model="form.registerName"
                label="Register name"
                placeholder="POS-01"
                hint="Printed on sales, exchange, and end-of-shift receipts."
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-textarea v-model="form.returnPolicy" label="Return policy" rows="3" variant="outlined" density="compact" class="mb-2" />
              <v-select
                v-model="form.socialPlatforms"
                :items="socialPlatformOptions"
                label="Social platforms"
                multiple
                chips
                closable-chips
                hide-selected
                variant="outlined"
                density="compact"
                class="mb-2"
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
              <div v-for="platform in form.socialPlatforms" :key="platform" class="mb-2">
                <v-text-field
                  v-model="form.socialHandles[platform]"
                  :label="`${platformLabel(platform)} handle`"
                  :prepend-inner-icon="platformIcon(platform)"
                  variant="outlined"
                  density="compact"
                />
              </div>
              <v-row density="comfortable">
                <v-col cols="12"><v-text-field v-model="form.website" label="Website" variant="outlined" density="compact" /></v-col>
              </v-row>
              <v-select v-model="form.receiptFont" :items="['Belleza', 'Helvetica', 'Times', 'Courier']" label="Receipt font" hint="Belleza is bundled for offline printing." persistent-hint variant="outlined" density="compact" class="mb-2" />
              <v-select v-model="form.receiptBodyFont" :items="['Printer Font A', 'Courier', 'Helvetica', 'Times', 'Belleza']" label="Body font" hint="Printer Font A uses the thermal printer's native font." persistent-hint variant="outlined" density="compact" class="mb-2" />
              <v-row density="comfortable">
                <v-col cols="6"><v-text-field v-model.number="form.receiptTitleSize" label="Market name size" type="number" min="8" max="40" variant="outlined" density="compact" /></v-col>
                <v-col cols="6"><v-text-field v-model.number="form.receiptBodySize" label="Body size" type="number" min="6" max="24" variant="outlined" density="compact" /></v-col>
              </v-row>
              <v-switch v-model="form.receiptCutoff" label="Cut paper after printing" color="primary" hide-details />
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
              <v-btn
                variant="tonal"
                color="primary"
                :loading="samplePrinting"
                @click="printCloseShiftSample"
              >
                <v-icon start icon="mdi-printer-eye" /> Print current close-shift sample
              </v-btn>
              <v-spacer />
              <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings"><v-icon start icon="mdi-content-save-outline" /> Save changes</v-btn>
            </v-card-actions>
          </v-card>

        </div>
      </v-window-item>

      <v-window-item value="coupons">
        <v-card class="settings-card" max-width="720">
          <v-card-title>Coupon types</v-card-title>
          <v-card-text>
            <div class="d-flex ga-2 align-start">
              <v-text-field v-model="couponTypeForm.name" label="Type name" variant="outlined" density="compact" />
              <v-text-field v-model.number="couponTypeForm.value" label="Value (USD)" type="number" min="0.01" step="0.01" variant="outlined" density="compact" />
              <v-btn color="primary" :loading="couponTypeSaving" @click="addCouponType">Add</v-btn>
            </div>
            <v-checkbox
              v-model="couponTypeForm.referenceOnly"
              class="mt-n3"
              density="compact"
              hide-details
              label="Reference-only (do not validate against an issued coupon)"
            />
            <div class="d-flex ga-2 align-start mb-2">
              <v-text-field v-model="couponCodeForm" label="New coupon code" variant="outlined" density="compact" />
              <v-select v-model="couponIssueTypeId" :items="couponTypes" item-title="name" item-value="id" label="Type" variant="outlined" density="compact" />
              <v-btn color="secondary" :loading="couponIssuing" @click="issueCoupon">Issue</v-btn>
            </div>
            <v-list density="compact">
              <v-list-item v-for="couponType in couponTypes" :key="couponType.id" :title="couponType.name" :subtitle="`${(Number(couponType.valueCents) / 100).toFixed(2)} ${couponType.currencyCode}${couponType.referenceOnly ? ' · Reference-only' : ''}`" />
              <v-list-item v-if="!couponTypes.length" title="No coupon types configured" />
            </v-list>
          </v-card-text>
        </v-card>
      </v-window-item>

      <v-window-item value="admin">
        <div class="settings-grid settings-grid--admin">
          <v-card class="settings-card">
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

          <v-card class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-lan-connect" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Remote admin access</div>
                <div class="text-caption text-medium-emphasis">Allow the admin TUI to manage this POS remotely.</div>
              </div>
            </v-card-title>
            <v-card-text>
              <v-switch v-model="form.adminEnabled" label="Enable remote admin API" color="primary" hide-details class="mb-3" />
              <v-text-field v-model="form.adminListen" label="Listen address" hint="Use 0.0.0.0:8787 for LAN access." persistent-hint variant="outlined" density="compact" class="mb-2" />
              <v-text-field
                v-model="form.adminToken"
                label="Admin token"
                hint="Required when listening beyond localhost."
                persistent-hint
                variant="outlined"
                density="compact"
                :type="showAdminToken ? 'text' : 'password'"
                :append-inner-icon="showAdminToken ? 'mdi-eye' : 'mdi-eye-off'"
                @click:append-inner="showAdminToken = !showAdminToken"
              />
            </v-card-text>
            <v-card-actions class="px-4 pb-4"><v-spacer /><v-btn color="primary" variant="flat" :loading="adminSettingsSaving" @click="saveAdminSettings"><v-icon start icon="mdi-content-save-outline" /> Save admin settings</v-btn></v-card-actions>
          </v-card>

          <v-form @submit.prevent="changePassword">
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
                  label="Current Password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                  :append-inner-icon="showPw ? 'mdi-eye' : 'mdi-eye-off'"
                  @click:append-inner="showPw = !showPw"
                />
                <v-text-field
                  v-model="passwordForm.newPass"
                  label="New Password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                />
                <v-text-field
                  v-model="passwordForm.confirm"
                  label="Confirm New Password"
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

          <v-card class="settings-card settings-card--wide">
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
              :items-per-page="10"
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

          <v-card class="settings-card settings-card--wide">
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
            <v-data-table :headers="salespersonHeaders" :items="salespersons" density="comfortable" hover>
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
            label="Username"
            variant="outlined"
            density="compact"
            class="mb-2"
          />
          <v-text-field
            v-model="userForm.password"
            label="Password"
            variant="outlined"
            density="compact"
            class="mb-2"
            :type="showUpw ? 'text' : 'password'"
            :append-inner-icon="showUpw ? 'mdi-eye' : 'mdi-eye-off'"
            @click:append-inner="showUpw = !showUpw"
            :placeholder="editingUser ? 'Leave blank to keep current' : ''"
          />
          <v-select
            v-model="userForm.roleId"
            :items="roles"
            item-title="name"
            item-value="id"
            label="Role"
            variant="outlined"
            density="compact"
            class="mb-2"
          />
          <v-text-field v-model="userForm.attendanceUserId" label="Akuvox UserID (leave blank for visitors)" variant="outlined" density="compact" />
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
          <v-text-field v-model="salespersonName" label="Full name" variant="outlined" density="compact" autofocus />
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

    <v-snackbar v-model="snackbar.show" class="app-snackbar" timeout="3500" location="top" variant="flat">
      {{ snackbar.text }}
    </v-snackbar>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from "vue";
import {
  GetAppSettings,
  UpdateSettings,
  UpdateAttendanceConfig,
  SetCustomItemDiscountsEnabled,
  SetCustomItemPricesEnabled,
  SetPerUnitItemPricesEnabled,
  UpdateUserAttendanceID,
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
  CreateCouponType,
  CreateCoupon,
  ListSalespersons,
  CreateSalesperson,
  UpdateSalesperson,
  DeleteSalesperson,
} from "../../wailsjs/go/main/App";
import { useAuthStore } from "@/stores/auth";
import { animationsEnabled, setAnimationsEnabled } from "@/lib/display";

const auth = useAuthStore();
const tab = ref("general");

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
  receiptEnabled: false,
  salesInventoryId: 0,
  exchangeRate: 90000,
  maxOverpayment: 1,
  allowOverpayment: false,
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
  adminEnabled: false,
  adminListen: "127.0.0.1:8787",
  adminToken: "",
  akuvoxIP: "",
  akuvoxUsername: "",
  akuvoxPassword: "",
  attendanceEnabled: false,
});
const roundingModes = [
  { title: "Half up (standard)", value: "half_up" },
  { title: "Always down", value: "down" },
  { title: "Always up", value: "up" },
];
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
const couponTypeForm = reactive({ name: "", value: 50, referenceOnly: false });
const couponTypeSaving = ref(false);
const couponCodeForm = ref("");
const couponIssueTypeId = ref<number | null>(null);
const couponIssuing = ref(false);
const pricingSettingsSaving = ref(false);
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
  { title: "", key: "actions", sortable: false, align: "end" as const },
];
const salespersonDialog = ref(false);
const editingSalesperson = ref<any>(null);
const salespersonName = ref("");
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
}

async function addCouponType() {
  if (!couponTypeForm.name.trim() || Number(couponTypeForm.value) <= 0) return;
  couponTypeSaving.value = true;
  try {
    await CreateCouponType(couponTypeForm.name.trim(), "USD", Math.round(Number(couponTypeForm.value) * 100), couponTypeForm.referenceOnly);
    couponTypeForm.name = "";
    couponTypeForm.referenceOnly = false;
    await loadCouponTypes();
    notify("Coupon type added");
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
        receiptEnabled: Boolean(s.receiptEnabled),
        salesInventoryId: Number(s.salesInventoryId ?? 0),
        exchangeRate: Number(s.exchangeRate ?? 90000),
        maxOverpayment: Number(s.maxOverpaymentCents ?? 0) / 100,
        allowOverpayment: Number(s.maxOverpaymentCents ?? 0) > 0,
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
        akuvoxIP: s.attendance?.ip ?? "",
        akuvoxUsername: s.attendance?.username ?? "",
        attendanceEnabled: Boolean(s.attendance?.enabled),
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

onMounted(async () => {
  await loadCouponTypes();
  await loadSettings();
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
      Boolean(form.receiptEnabled),
      Number(form.salesInventoryId) > 0 ? Number(form.salesInventoryId) : 0,
      Number.isFinite(exchangeRate) && exchangeRate > 0 ? Math.round(exchangeRate) : 90000,
      maxOverpaymentCents,
    );
    await UpdateReceiptSettings(
      form.marketName.trim(), form.marketPhone.trim(), form.branchPhone.trim(), form.marketDescription.trim(), form.returnPolicy.trim(),
      form.website.trim(), form.instagram.trim(), JSON.stringify(form.socialPlatforms ?? []), JSON.stringify(form.socialHandles ?? {}),
      form.registerName.trim(), form.receiptFont, form.receiptBodyFont,
      Number(form.receiptTitleSize), Number(form.receiptBodySize),
      Boolean(form.receiptCutoff),
    );
    await UpdateAttendanceConfig(form.akuvoxIP, form.akuvoxUsername, form.akuvoxPassword, Boolean(form.attendanceEnabled));
    notify("Settings saved");
  } catch {
    notify("Failed to save", "red");
  }
  saving.value = false;
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
  salespersonDialog.value = true;
}

async function saveSalesperson() {
  if (!salespersonName.value.trim()) return;
  savingSalesperson.value = true;
  try {
    if (editingSalesperson.value) {
      await UpdateSalesperson(editingSalesperson.value.id, salespersonName.value);
    } else {
      await CreateSalesperson(salespersonName.value);
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
  padding: 24px;
}

.settings-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  max-width: 1100px;
  margin: 0 auto 24px;
}

.settings-heading-icon {
  opacity: 0.16;
}

.settings-tabs,
.settings-grid {
  max-width: 1100px;
  margin-inline: auto;
}

.settings-tabs {
  border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: start;
  gap: 16px;
}

.settings-grid--admin {
  grid-template-columns: minmax(280px, 0.8fr) minmax(0, 1.6fr);
}

.settings-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.settings-card :deep(.v-card-title) {
  padding: 20px 20px 8px;
}

.settings-card :deep(.v-card-text) {
  padding: 16px 20px 4px;
}

.settings-card :deep(.v-data-table-footer) {
  padding-inline: 20px;
}

.app-snackbar .v-snackbar__wrapper {
  background: rgba(var(--v-theme-surface-variant), 0.72) !important;
  color: rgb(var(--v-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}

@media (max-width: 900px) {
  .settings-grid,
  .settings-grid--admin {
    grid-template-columns: 1fr;
  }

  .settings-page {
    padding: 16px;
  }
}
</style>
