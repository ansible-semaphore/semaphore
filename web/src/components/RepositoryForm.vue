<template>
  <v-form
      ref="form"
      lazy-validation
      v-model="formValid"
      v-if="item != null && keys != null"
  >
    <v-alert
        :value="formError"
        color="error"
        class="pb-2"
    >{{ formError }}
    </v-alert>

    <v-text-field
        v-model.trim="item.name"
        label="Name"
        :rules="[v => !!v || 'Name is required']"
        required
        :disabled="formSaving"
    ></v-text-field>

    <v-text-field
        v-model.trim="item.git_url"
        label="URL or path"
        :rules="[
          v => !!v || 'Repository is required',
          v => getTypeOfUrl(v) != null || 'Incorrect URL',
        ]"
        required
        :disabled="formSaving"
        :hide-details="true"
    ></v-text-field>

    <div class="mt-1 mb-4">
      <span class="caption">git:</span>
      <v-chip
        v-for="x in ['ssh', 'https', 'file', 'git']"
        x-small
        class="ml-1"
        :color="type ===x ? 'primary' : ''"
        @click="setType(x)"
        style="font-weight: bold;"
        :key="x"
      >{{ x }}</v-chip>
      <span class="caption ml-3">local:</span>
      <v-chip
        x-small
        class="ml-1"
        :color="type ==='local' ? 'primary' : ''"
        @click="setType('local')"
        style="font-weight: bold;"
      >
        abs. path
      </v-chip>
    </div>

    <v-text-field
      v-model.trim="item.git_branch"
      label="Branch"
      :rules="[v => (!!v || type === 'local') || 'Branch is required']"
      required
      :disabled="formSaving || type === 'local'"
    ></v-text-field>

    <v-select
        v-model="item.ssh_key_id"
        label="Access Key"
        :items="keys"
        item-value="id"
        item-text="name"
        :rules="[v => !!v || 'Key is required']"
        required
        :disabled="formSaving"
    >
      <template v-slot:append-outer>
        <v-tooltip left color="black" content-class="opacity1">
          <template v-slot:activator="{ on, attrs }">
            <v-icon
              v-bind="attrs"
              v-on="on"
            >
              mdi-help-circle
            </v-icon>
          </template>
          <div class="py-4">
            <p>Credentials to access to the Git repository. It should be:</p>
            <ul>
              <li><code>SSH</code> if you use Git or SSH URL.</li>
              <li><code>None</code> if you use HTTPS or file URL.</li>
            </ul>
          </div>
        </v-tooltip>
      </template>
    </v-select>
  </v-form>
</template>
<script>
import axios from 'axios';
import ItemFormBase from '@/components/ItemFormBase';

export default {
  mixins: [ItemFormBase],
  data() {
    return {
      helpDialog: null,
      helpKey: null,

      keys: null,
      inventoryTypes: [{
        id: 'static',
        name: 'Static',
      }, {
        id: 'static-yaml',
        name: 'Static YAML',
      }, {
        id: 'file',
        name: 'File',
      }],
    };
  },
  async created() {
    this.keys = (await axios({
      keys: 'get',
      url: `/api/project/${this.projectId}/keys`,
      responseType: 'json',
    })).data;
  },
  computed: {
    type() {
      return this.getTypeOfUrl(this.item.git_url);
    },
  },

  methods: {
    getTypeOfUrl(url) {
      if (url == null || url === '') {
        return null;
      }

      if (url.startsWith('/')) {
        return 'local';
      }

      const m = url.match(/^(\w+):\/\//);

      if (m == null) {
        return 'ssh';
      }

      if (!['git', 'file', 'ssh', 'https'].includes(m[1])) {
        return null;
      }

      return m[1];
    },

    setType(type) {
      let url;

      const m = this.item.git_url.match(/^\w+:\/\/(.*)$/);
      if (m != null) {
        url = m[1];
      } else {
        url = this.item.git_url;
      }

      if (type === 'local') {
        url = url.startsWith('/') ? url : `/${url}`;
      } else {
        url = `${type}://${url}`;
      }

      this.item.git_url = url;
    },

    showHelpDialog(key) {
      this.helpKey = key;
      this.helpDialog = true;
    },

    getItemsUrl() {
      return `/api/project/${this.projectId}/repositories`;
    },

    getSingleItemUrl() {
      return `/api/project/${this.projectId}/repositories/${this.itemId}`;
    },
  },
};
</script>
