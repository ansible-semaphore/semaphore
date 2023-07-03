<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div v-if="items != null">
    <EditDialog
      v-model="editDialog"
      save-button-text="Save"
      title="Edit User"
      @save="loadItems()"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <UserForm
          :project-id="projectId"
          :item-id="itemId"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <YesNoDialog
      title="Delete user"
      text="Are you really want to delete this user?"
      v-model="deleteItemDialog"
      @yes="deleteItem(itemId)"
    />

    <v-toolbar flat >
      <v-btn
        icon
        class="mr-4"
        @click="returnToProjects()"
      >
        <v-icon>mdi-arrow-left</v-icon>
      </v-btn>
      <v-toolbar-title>Users</v-toolbar-title>
      <v-spacer></v-spacer>
      <v-btn
        color="primary"
        @click="editItem('new')"
      >New User</v-btn>
    </v-toolbar>

    <v-data-table
      :headers="headers"
      :items="items"
      :footer-props="{ itemsPerPageOptions: [20] }"
      :hide-default-footer="hideFooter"
      class="mt-4"
    >
      <template v-slot:item.external="{ item }">
        <v-icon v-if="item.external">mdi-checkbox-marked</v-icon>
        <v-icon v-else>mdi-checkbox-blank-outline</v-icon>
      </template>

      <template v-slot:item.alert="{ item }">
        <v-icon v-if="item.alert">mdi-checkbox-marked</v-icon>
        <v-icon v-else>mdi-checkbox-blank-outline</v-icon>
      </template>

      <template v-slot:item.admin="{ item }">
        <v-icon v-if="item.admin">mdi-checkbox-marked</v-icon>
        <v-icon v-else>mdi-checkbox-blank-outline</v-icon>
      </template>

      <template v-slot:item.actions="{ item }">
        <div style="white-space: nowrap">
          <v-btn
            icon
            class="mr-1"
            @click="askDeleteItem(item.id)"
            :disabled="item.id === userId"
          >
            <v-icon>mdi-delete</v-icon>
          </v-btn>

          <v-btn
            icon
            class="mr-1"
            @click="editItem(item.id)"
          >
            <v-icon>mdi-pencil</v-icon>
          </v-btn>
        </div>
      </template>
    </v-data-table>
  </div>
</template>
<script>
import EventBus from '@/event-bus';
import YesNoDialog from '@/components/YesNoDialog.vue';
import ItemListPageBase from '@/components/ItemListPageBase';
import EditDialog from '@/components/EditDialog.vue';
import UserForm from '@/components/UserForm.vue';

export default {
  mixins: [ItemListPageBase],

  components: {
    YesNoDialog,
    UserForm,
    EditDialog,
  },

  methods: {
    getHeaders() {
      return [{
        text: 'Name',
        value: 'name',
        width: '50%',
      },
      {
        text: 'Username',
        value: 'username',
      },
      {
        text: 'Email',
        value: 'email',
      },
      {
        text: 'Alert',
        value: 'alert',
      },
      {
        text: 'Admin',
        value: 'admin',
      },
      {
        text: 'External',
        value: 'external',
        width: '50%',
      },
      {
        text: 'Actions',
        value: 'actions',
        sortable: false,
      }];
    },

    async returnToProjects() {
      EventBus.$emit('i-open-last-project');
    },

    getItemsUrl() {
      return '/api/users';
    },

    getSingleItemUrl() {
      return `/api/users/${this.itemId}`;
    },

    getEventName() {
      return 'i-user';
    },
  },
};
</script>
