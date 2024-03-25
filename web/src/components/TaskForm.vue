<template>
  <v-form
    ref="form"
    lazy-validation
    v-model="formValid"
    v-if="isLoaded()"
  >
    <v-alert
      :value="formError"
      color="error"
      class="pb-2"
    >{{ formError }}
    </v-alert>

    <v-alert
      color="blue"
      dark
      icon="mdi-source-fork"
      dismissible
      v-model="commitAvailable"
      prominent
    >
      <div
        style="font-weight: bold;"
      >{{ (item.commit_hash || '').substr(0, 10) }}
      </div>
      <div v-if="sourceTask && sourceTask.commit_message">{{ sourceTask.commit_message }}</div>
    </v-alert>

    <v-select
      v-if="template.type === 'deploy'"
      v-model="item.build_task_id"
      :label="$t('buildVersion')"
      :items="buildTasks"
      item-value="id"
      :item-text="(itm) => getTaskMessage(itm)"
      :rules="[v => !!v || $t('build_version_required')]"
      required
      :disabled="formSaving"
    />

    <v-text-field
      v-model="item.message"
      :label="$t('messageOptional')"
      :disabled="formSaving"
    />

    <v-text-field
      v-for="(v) in template.survey_vars || []"
      :key="v.name"
      :label="v.title"
      :hint="v.description"
      v-model="editedEnvironment[v.name]"
      :required="v.required"
      :type="v.type === 'secret' ?'password' : 'text' "
      :rules="[
          val => !v.required || !!val || v.title + $t('isRequired'),
          val => !val || v.type !== 'int' || /^\d+$/.test(val) ||
          v.title + ' ' + $t('mustBeInteger'),
        ]"
    />

    <v-row no-gutters class="mt-6">
      <v-col cols="12" sm="6">
        <v-checkbox class="mt-0" v-model="item.debug">
          <template v-slot:label>
            <div class="text-no-wrap">{{ $t('debug') }} <code>--vvvv</code></div>
          </template>
        </v-checkbox>
      </v-col>
      <v-col cols="12" sm="6">
        <v-checkbox class="mt-0" v-model="item.dry_run">
          <template v-slot:label>
            <div class="text-no-wrap">{{ $t('dryRun') }} <code>--check</code></div>
          </template>
        </v-checkbox>
      </v-col>
      <v-col cols="12" sm="6">
        <v-checkbox class="mt-0" v-model="item.diff">
          <template v-slot:label>
            <div class="text-no-wrap">{{ $t('diff') }} <code>--diff</code></div>
          </template>
        </v-checkbox>
      </v-col>
    </v-row>

    <div class="mt-4" v-if="!advancedOptions">
      <a @click="advancedOptions = true">
        {{ $t('advanced') }}
        <v-icon style="transform: translateY(-1px)">mdi-chevron-right</v-icon>
      </a>
    </div>

    <div class="mt-4" v-else>
      <a @click="advancedOptions = false">
        {{ $t('hide') }}
        <v-icon style="transform: translateY(-1px)">mdi-chevron-up</v-icon>
      </a>
    </div>

    <v-alert
      v-if="advancedOptions && !template.allow_override_args_in_task"
      color="info"
      dense
      text
      class="mb-2"
    >
      {{ $t('pleaseAllowOverridingCliArgumentInTaskTemplateSett') }}<br>
      <div style="position: relative; margin-top: 10px;">
        <video
          autoplay
          muted
          style="width: 100%; border-radius: 4px;"
        >
          <source
            src="/allow-override-cli-args-in-task.mp4"
            type="video/mp4"/>
        </video>
      </div>
    </v-alert>

    <codemirror
      class="mt-4"
      v-if="advancedOptions && template.allow_override_args_in_task"
      :style="{ border: '1px solid lightgray' }"
      v-model="item.arguments"
      :options="cmOptions"
      :placeholder="$t('cliArgsJsonArrayExampleIMyinventoryshPrivatekeythe')"
    />

  </v-form>
</template>
<script>
/* eslint-disable import/no-extraneous-dependencies,import/extensions */

import ItemFormBase from '@/components/ItemFormBase';
import axios from 'axios';
import { codemirror } from 'vue-codemirror';
import 'codemirror/lib/codemirror.css';
import 'codemirror/mode/vue/vue.js';
import 'codemirror/addon/lint/json-lint.js';
import 'codemirror/addon/display/placeholder.js';

export default {
  mixins: [ItemFormBase],
  props: {
    templateId: Number,
    sourceTask: Object,
  },
  components: {
    codemirror,
  },
  data() {
    return {
      template: null,
      buildTasks: null,
      commitAvailable: null,
      editedEnvironment: null,
      cmOptions: {
        tabSize: 2,
        mode: 'application/json',
        lineNumbers: true,
        line: true,
        lint: true,
        indentWithTabs: false,
      },
      advancedOptions: false,
    };
  },
  watch: {
    needReset(val) {
      if (val) {
        this.item.template_id = this.templateId;
      }
    },

    templateId(val) {
      this.item.template_id = val;
    },

    sourceTask(val) {
      this.assignItem(val);
    },

    commitAvailable(val) {
      if (val == null) {
        this.commit_hash = null;
      }
    },
  },
  methods: {
    getTaskMessage(task) {
      let buildTask = task;

      while (buildTask.version == null && buildTask.build_task != null) {
        buildTask = buildTask.build_task;
      }

      if (!buildTask) {
        return '';
      }

      return buildTask.version + (buildTask.message ? ` — ${buildTask.message}` : '');
    },

    assignItem(val) {
      const v = val || {};

      if (this.item == null) {
        this.item = {};
      }

      Object.keys(v).forEach((field) => {
        this.item[field] = v[field];
      });

      this.editedEnvironment = JSON.parse(v.environment || '{}');
      this.commitAvailable = v.commit_hash != null;
    },

    isLoaded() {
      return this.item != null
        && this.template != null
        && this.buildTasks != null;
    },

    beforeSave() {
      this.item.environment = JSON.stringify(this.editedEnvironment);
    },

    async afterLoadData() {
      this.assignItem(this.sourceTask);

      this.item.template_id = this.templateId;

      this.advancedOptions = this.item.arguments != null;

      this.template = (await axios({
        keys: 'get',
        url: `/api/project/${this.projectId}/templates/${this.templateId}`,
        responseType: 'json',
      })).data;

      this.buildTasks = this.template.type === 'deploy' ? (await axios({
        keys: 'get',
        url: `/api/project/${this.projectId}/templates/${this.template.build_template_id}/tasks?status=success`,
        responseType: 'json',
      })).data.filter((task) => task.status === 'success') : [];

      if (this.item.build_task_id == null
        && this.buildTasks.length > 0
        && this.buildTasks.length > 0) {
        this.item.build_task_id = this.buildTasks[0].id;
      }
    },

    getItemsUrl() {
      return `/api/project/${this.projectId}/tasks`;
    },
  },
};
</script>
