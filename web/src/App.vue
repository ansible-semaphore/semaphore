<template>
  <v-app v-if="state === 'success'" class="app">
    <EditDialog
      v-model="passwordDialog"
      save-button-text="Save"
      :title="$t('changePassword')"
      v-if="user"
      event-name="i-user"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <ChangePasswordForm
          :project-id="projectId"
          :item-id="user.id"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <EditDialog
      v-model="userDialog"
      save-button-text="Save"
      :title="$t('editUser')"
      v-if="user"
      event-name="i-user"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <UserForm
          :project-id="projectId"
          :item-id="user.id"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
          :is-admin="user.admin"
        />
      </template>
    </EditDialog>

    <EditDialog
      v-model="taskLogDialog"
      save-button-text="Delete"
      :max-width="1000"
      :hide-buttons="true"
      @close="onTaskLogDialogClosed()"
    >
      <template v-slot:title={}>
        <div class="text-truncate" style="max-width: calc(100% - 36px);">
          <router-link
            class="breadcrumbs__item breadcrumbs__item--link"
            :to="`/project/${projectId}/templates/${template ? template.id : null}`"
            @click="taskLogDialog = false"
          >{{ template ? template.name : null }}
          </router-link>
          <v-icon>mdi-chevron-right</v-icon>
          <span class="breadcrumbs__item">{{ $t('task', {expr: task ? task.id : null}) }}</span>
        </div>

        <v-spacer></v-spacer>
        <v-btn
          icon
          @click="taskLogDialog = false; onTaskLogDialogClosed()"
        >
          <v-icon>mdi-close</v-icon>
        </v-btn>
      </template>
      <template v-slot:form="{}">
        <TaskLogView :project-id="projectId" :item-id="task ? task.id : null"/>
      </template>
    </EditDialog>

    <EditDialog
      v-model="newProjectDialog"
      save-button-text="Create"
      :title="$t('newProject')"
      event-name="i-project"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <ProjectForm
          v-if="newProjectType === ''"
          item-id="new"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <v-snackbar
      v-model="snackbar"
      :color="snackbarColor"
      :timeout="3000"
      top
    >
      {{ snackbarText }}
      <v-btn
        text
        @click="snackbar = false"
      >
        {{ $t('close') }}
      </v-btn>
    </v-snackbar>

    <v-navigation-drawer
      app
      dark
      :color="darkMode ? '#003236' : '#005057'"
      fixed
      width="260"
      v-model="drawer"
      mobile-breakpoint="960"
      v-if="$route.path.startsWith('/project/')"
    >
      <v-menu bottom max-width="235" max-height="100%" v-if="project">
        <template v-slot:activator="{ on, attrs }">
          <v-list class="pa-0 overflow-y-auto">
            <v-list-item
              key="project"
              class="app__project-selector"
              v-bind="attrs"
              v-on="on"
            >
              <v-list-item-icon>
                <v-avatar
                  :color="getProjectColor(project)"
                  size="24"
                  style="font-size: 13px; font-weight: bold;"
                >
                  <span class="white--text">{{ getProjectInitials(project) }}</span>
                </v-avatar>
              </v-list-item-icon>

              <v-list-item-content>
                <v-list-item-title class="app__project-selector-title">
                  {{ project.name }}
                </v-list-item-title>
                <v-list-item-subtitle>{{ userRole.role }}</v-list-item-subtitle>
              </v-list-item-content>

              <v-list-item-icon>
                <v-icon>mdi-chevron-down</v-icon>
              </v-list-item-icon>
            </v-list-item>
          </v-list>
        </template>
        <v-list>
          <v-list-item
            v-for="(item, i) in projects"
            :key="i"
            :to="`/project/${item.id}`"
            @click="selectProject(item.id)"
          >
            <v-list-item-icon>
              <v-avatar
                :color="getProjectColor(item)"
                size="24"
                style="font-size: 13px; font-weight: bold;"
              >
                <span class="white--text">{{ getProjectInitials(item) }}</span>
              </v-avatar>
            </v-list-item-icon>
            <v-list-item-content>{{ item.name }}</v-list-item-content>
          </v-list-item>

          <v-list-item
            @click="newProjectDialog = true; newProjectType = '';"
            v-if="user.can_create_project"
          >
            <v-list-item-icon>
              <v-icon>mdi-plus</v-icon>
            </v-list-item-icon>

            <v-list-item-content>
              {{ $t('newProject2') }}
            </v-list-item-content>
          </v-list-item>

          <v-list-item @click="restoreProject" v-if="user.can_create_project">
            <v-list-item-icon>
              <v-icon>mdi-backup-restore</v-icon>
            </v-list-item-icon>

            <v-list-item-content>
              {{ $t('restoreProject') }}
            </v-list-item-content>
          </v-list-item>
        </v-list>
      </v-menu>

      <v-list class="pt-0" v-if="!project">
        <v-list-item key="new_project" :to="`/project/new`">
          <v-list-item-icon>
            <v-icon>mdi-plus</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('newProject') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>
      </v-list>

      <v-list class="pt-0" v-if="project">

        <v-list-item key="dashboard" :to="`/project/${projectId}/history`">
          <v-list-item-icon>
            <v-icon>mdi-view-dashboard</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('dashboard') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="project.type === ''" key="templates" :to="templatesUrl">
          <v-list-item-icon>
            <v-icon>mdi-check-all</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('taskTemplates') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item
          v-if="project.type === ''"
          key="inventory"
          :to="`/project/${projectId}/inventory`"
        >
          <v-list-item-icon>
            <v-icon>mdi-monitor-multiple</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('inventory') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item
          v-if="project.type === ''"
          key="environment"
          :to="`/project/${projectId}/environment`"
        >
          <v-list-item-icon>
            <v-icon>mdi-code-braces</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('environment') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item
          v-if="project.type === ''"
          key="keys"
          :to="`/project/${projectId}/keys`"
        >
          <v-list-item-icon>
            <v-icon>mdi-key-change</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('keyStore') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item
          v-if="project.type === ''"
          key="repositories"
          :to="`/project/${projectId}/repositories`"
        >
          <v-list-item-icon>
            <v-icon>mdi-git</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('repositories') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item
          v-if="project.type === ''"
          key="integrations"
          :to="`/project/${projectId}/integrations`"
        >
          <v-list-item-icon>
            <v-icon>mdi-connection</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>Integrations</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item key="team" :to="`/project/${projectId}/team`">
          <v-list-item-icon>
            <v-icon>mdi-account-multiple</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('team') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>
      </v-list>

      <template v-slot:append>
        <v-list class="pa-0">
          <v-list-item>
            <v-switch
              v-model="darkMode"
              inset
              :label="$t('darkMode')"
              persistent-hint
            ></v-switch>

            <v-spacer/>

            <v-menu top min-width="150" max-width="235" nudge-top="12" :position-x="50" absolute>
              <template v-slot:activator="{on, attrs}">
                <v-btn
                  icon
                  x-large
                  v-bind="attrs"
                  v-on="on"
                >
                  <span style="font-size: 30px;">{{ lang.flag }}</span>
                </v-btn>
              </template>

              <v-list dense>
                <v-list-item
                  v-for="lang in languages"
                  :key="lang.id"
                  @click="selectLanguage(lang.id)"
                >

                  <v-list-item-icon>
                    {{ lang.flag }}
                  </v-list-item-icon>

                  <v-list-item-content>
                    <v-list-item-title>{{ lang.title }}</v-list-item-title>
                  </v-list-item-content>

                </v-list-item>
              </v-list>
            </v-menu>

          </v-list-item>

          <v-menu top max-width="235" nudge-top="12">
            <template v-slot:activator="{ on, attrs }">
              <v-list-item
                key="project"
                v-bind="attrs"
                v-on="on"
              >
                <v-list-item-icon>
                  <v-icon>mdi-account</v-icon>
                </v-list-item-icon>

                <v-list-item-content>
                  <v-list-item-title>
                    {{ user.name }}
                  </v-list-item-title>

                </v-list-item-content>

                <v-list-item-action>
                  <v-chip color="red" v-if="user.admin" small>admin</v-chip>
                </v-list-item-action>
              </v-list-item>
            </template>

            <v-list>
              <v-list-item key="version">
                <v-list-item-icon>
                  <v-icon>mdi-information-variant</v-icon>
                </v-list-item-icon>

                <v-list-item-content>
                  {{ systemInfo.version }}
                </v-list-item-content>
              </v-list-item>

              <v-divider/>

              <v-list-item key="users" to="/users" v-if="user.admin">
                <v-list-item-icon>
                  <v-icon>mdi-account-multiple</v-icon>
                </v-list-item-icon>

                <v-list-item-content>
                  {{ $t('users') }}
                </v-list-item-content>
              </v-list-item>

              <v-list-item key="edit" @click="userDialog = true">
                <v-list-item-icon>
                  <v-icon>mdi-pencil</v-icon>
                </v-list-item-icon>

                <v-list-item-content>
                  {{ $t('editAccount') }}
                </v-list-item-content>
              </v-list-item>

              <v-list-item key="sign_out" @click="signOut()">
                <v-list-item-icon>
                  <v-icon>mdi-exit-to-app</v-icon>
                </v-list-item-icon>

                <v-list-item-content>
                  {{ $t('signOut') }}
                </v-list-item-content>
              </v-list-item>
            </v-list>
          </v-menu>

        </v-list>

      </template>
    </v-navigation-drawer>

    <v-main>
      <router-view
        :projectId="projectId"
        :projectType="(project || {}).type || ''"
        :userPermissions="(userRole || {}).permissions"
        :userRole="(userRole || {}).role"
        :userId="(user || {}).id"
        :isAdmin="(user || {}).admin"
        :user="user"
      ></router-view>
    </v-main>

  </v-app>
  <v-app v-else-if="state === 'loading'">
    <v-main>
      <v-container
        fluid
        fill-height
        align-center
        justify-center
        class="pa-0"
      >
        <v-progress-circular
          :size="70"
          color="primary"
          indeterminate
        ></v-progress-circular>
      </v-container>
    </v-main>
  </v-app>
  <v-app v-else-if="state === 'error'">
    <v-main>
      <v-container
        fluid
        flex-column
        fill-height
        align-center
        justify-center
        class="pa-0 text-center"
      >
        <v-alert text color="error" class="d-inline-block">
          <h3 class="headline">
            {{ $t('error') }}
          </h3>
          {{ snackbarText }}
        </v-alert>
        <div class="mb-6">
          <v-btn text color="blue darken-1" @click="refreshPage()">
            <v-icon left>mdi-refresh</v-icon>
            {{ $t('refreshPage') }}
          </v-btn>
          <v-btn text color="blue darken-1" @click="signOut()">
            <v-icon left>mdi-exit-to-app</v-icon>
            {{ $t('relogin') }}
          </v-btn>
        </div>
      </v-container>
    </v-main>
  </v-app>
  <v-app v-else></v-app>
</template>
<style lang="scss">
.v-dialog > .v-card > .v-card__title {
  flex-wrap: nowrap;
  overflow: hidden;

  & * {
    white-space: nowrap;
  }
}

.v-data-table tbody tr.v-data-table__expanded__content {
  box-shadow: none !important;

}

.v-data-table a {
  text-decoration-line: none;

  &:hover {
    text-decoration-line: underline;
  }
}

.breadcrumbs__item--link {
  text-decoration-line: none;

  &:hover {
    text-decoration-line: underline;
  }
}

.breadcrumbs__separator {
  padding: 0 10px;
}

.app__project-selector {
  height: 64px;

  & > .v-list-item__content {
    padding: 0;
  }

  .v-list-item__icon {
    margin-top: 20px !important;
  }
}

.app__project-selector-title {
  font-size: 1.25rem !important;
  font-weight: bold;
}

.v-application--is-ltr .v-list-item__action:first-child,
.v-application--is-ltr .v-list-item__icon:first-child {
  margin-right: 16px !important;
}

.v-toolbar__content {
  height: 64px !important;
}

.v-data-table-header {
}

.theme--light.v-data-table > .v-data-table__wrapper > table > thead > tr:last-child > th {
  text-transform: uppercase;
  white-space: nowrap;
}

.v-data-table > .v-data-table__wrapper > table > tbody > tr {
  background: transparent !important;

  & > td {
    white-space: nowrap;
  }

  & > td:first-child {
    //font-weight: bold !important;
  }
}

.v-data-table > .v-data-table__wrapper > table > tbody > tr > th,
.v-data-table > .v-data-table__wrapper > table > thead > tr > th,
.v-data-table > .v-data-table__wrapper > table > tfoot > tr > th,
.v-data-table > .v-data-table__wrapper > table > tbody > tr > td {
  font-size: 1rem !important;
}

.v-data-footer {
  font-size: 1rem !important;
}

.v-toolbar__title {
  font-weight: bold !important;
}

.v-app-bar__nav-icon {
  margin-left: 0 !important;
}

.v-toolbar__title:not(:first-child) {
  margin-left: 10px !important;
}

@media (min-width: 960px) {
  .v-app-bar__nav-icon {
    display: none !important;
  }

  .v-toolbar__title:not(:first-child) {
    padding-left: 0 !important;
    margin-left: 0 !important;
  }
}

</style>

<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';
import EditDialog from '@/components/EditDialog.vue';
import TaskLogView from '@/components/TaskLogView.vue';
import ProjectForm from '@/components/ProjectForm.vue';
import UserForm from '@/components/UserForm.vue';
import ChangePasswordForm from '@/components/ChangePasswordForm.vue';
import EventBus from '@/event-bus';
import socket from '@/socket';

const PROJECT_COLORS = [
  'red',
  'blue',
  'orange',
  'green',
];

const LANGUAGES = {
  en: {
    flag: '🇺🇸',
    title: 'English',
  },
  es: {
    flag: '🇨🇱',
    title: 'Español',
  },
  ru: {
    flag: '🇷🇺',
    title: 'Russian',
  },
  de: {
    flag: '🇩🇪',
    title: 'German',
  },
  zh_cn: {
    flag: '🇨🇳',
    title: '中文(大陆)',
  },
  zh_tw: {
    flag: '🇹🇼',
    title: '中文(台灣)',
  },
  fr: {
    flag: '🇫🇷',
    title: 'French',
  },
  it: {
    flag: '🇮🇹',
    title: 'Italian',
  },
  pl: {
    flag: '🇵🇱️',
    title: 'Polish',
  },
  pt: {
    flag: '🇵🇹',
    title: 'Portuguese',
  },
  pt_br: {
    flag: '🇧🇷',
    title: 'Português do Brasil',
  },
};

function getLangInfo(locale) {
  let res = LANGUAGES[locale];

  // failback short i18n
  if (!res) {
    res = LANGUAGES[locale.split('_')[0]];
  }

  if (!res) {
    res = LANGUAGES.en;
  }

  return res;
}

function getSystemLang() {
  const locale = navigator.language.replace('-', '_').toLocaleLowerCase();

  return getLangInfo(locale || 'en');
}

export default {
  name: 'App',
  components: {
    ChangePasswordForm,
    UserForm,
    EditDialog,
    TaskLogView,
    ProjectForm,
  },
  data() {
    return {
      drawer: null,
      user: null,
      userRole: null,
      systemInfo: null,
      state: 'loading',
      snackbar: false,
      snackbarText: '',
      snackbarColor: '',
      projects: null,
      newProjectDialog: null,
      newProjectType: '',
      userDialog: null,
      passwordDialog: null,

      taskLogDialog: null,
      task: null,
      template: null,
      darkMode: false,
      languages: [
        {
          id: '',
          flag: getSystemLang().flag,
          title: 'System',
        },
        ...Object.keys(LANGUAGES).map((lang) => ({
          id: lang,
          ...LANGUAGES[lang],
        })),
      ],
    };
  },

  watch: {
    async projects(val) {
      if (val.length === 0
        && this.$route.path.startsWith('/project/')
        && this.$route.path !== '/project/new') {
        await this.$router.push({ path: '/project/new' });
      }
    },

    async $route(val) {
      if (val.query.t == null) {
        this.taskLogDialog = false;
      } else {
        const taskId = parseInt(this.$route.query.t || '', 10);
        if (taskId) {
          EventBus.$emit('i-show-task', { taskId });
        }
      }
    },

    darkMode(val) {
      this.$vuetify.theme.dark = val;
      if (val) {
        localStorage.setItem('darkMode', '1');
      } else {
        localStorage.removeItem('darkMode');
      }
    },
  },

  computed: {

    lang() {
      const locale = localStorage.getItem('lang');

      if (!locale) {
        return getSystemLang();
      }

      return getLangInfo(locale || 'en');
    },

    projectId() {
      return parseInt(this.$route.params.projectId, 10) || null;
    },

    project() {
      if (this.projects == null) {
        return null;
      }
      return this.projects.find((x) => x.id === this.projectId);
    },

    isAuthenticated() {
      return document.cookie.includes('semaphore=');
    },

    templatesUrl() {
      let viewId = localStorage.getItem(`project${this.projectId}__lastVisitedViewId`);
      if (viewId) {
        viewId = parseInt(viewId, 10);
        if (!Number.isNaN(viewId)) {
          return `/project/${this.projectId}/views/${viewId}/templates`;
        }
      }
      return `/project/${this.projectId}/templates`;
    },
  },

  async created() {
    if (!this.isAuthenticated) {
      if (this.$route.path !== '/auth/login') {
        await this.$router.push({ path: '/auth/login' });
      }
      this.state = 'success';
      return;
    }

    if (localStorage.getItem('darkMode') === '1') {
      this.darkMode = true;
    }

    try {
      await this.loadData();
      this.state = 'success';
    } catch (err) {
      EventBus.$emit('i-snackbar', {
        color: 'error',
        text: getErrorMessage(err),
      });
      this.state = 'error';
      socket.stop();
    }
  },

  mounted() {
    EventBus.$on('i-snackbar', (e) => {
      this.snackbar = true;
      this.snackbarColor = e.color;
      this.snackbarText = e.text;
    });

    EventBus.$on('i-account-change', async () => {
      await this.loadUserInfo();
    });

    EventBus.$on('i-show-drawer', async () => {
      this.drawer = true;
    });

    EventBus.$on('i-show-task', async (e) => {
      if (parseInt(this.$route.query.t || '', 10) !== e.taskId) {
        const query = { ...this.$route.query, t: e.taskId };
        await this.$router.replace({ query });
      }

      this.task = (await axios({
        method: 'get',
        url: `/api/project/${this.projectId}/tasks/${e.taskId}`,
        responseType: 'json',
      })).data;

      this.template = (await axios({
        method: 'get',
        url: `/api/project/${this.projectId}/templates/${this.task.template_id}`,
        responseType: 'json',
      })).data;

      this.taskLogDialog = true;
    });

    EventBus.$on('i-open-last-project', async () => {
      await this.trySelectMostSuitableProject();
    });

    EventBus.$on('i-user', async (e) => {
      let text;

      switch (e.action) {
        case 'new':
          text = `User ${e.item.name} created`;
          break;
        case 'edit':
          text = `User ${e.item.name} saved`;
          break;
        case 'delete':
          text = `User ${e.item.name} deleted`;
          break;
        default:
          throw new Error('Unknown project action');
      }

      EventBus.$emit('i-snackbar', {
        color: 'success',
        text,
      });

      if (this.user && e.item.id === this.user.id) {
        await this.loadUserInfo();
      }
    });

    EventBus.$on('i-project', async (e) => {
      let text;

      const project = this.projects.find((p) => p.id === e.item.id) || e.item;
      const projectName = project.name || `#${project.id}`;

      switch (e.action) {
        case 'new':
          text = `Project ${projectName} created`;
          break;
        case 'edit':
          text = `Project ${projectName} saved`;
          break;
        case 'delete':
          text = `Project ${projectName} deleted`;
          break;
        default:
          throw new Error('Unknown project action');
      }

      EventBus.$emit('i-snackbar', {
        color: 'success',
        text,
      });

      await this.loadProjects();

      switch (e.action) {
        case 'new':
          await this.selectProject(e.item.id);
          break;
        case 'delete':
          if (this.projectId === e.item.id && this.projects.length > 0) {
            await this.selectProject(this.projects[0].id);
          }
          break;
        default:
          break;
      }
    });
  },

  methods: {

    selectLanguage(lang) {
      localStorage.setItem('lang', lang);
      window.location.reload();
    },

    async onTaskLogDialogClosed() {
      const query = { ...this.$route.query, t: undefined };
      await this.$router.replace({ query });
    },

    async loadData() {
      if (!socket.isRunning()) {
        socket.start();
      }

      await this.loadUserInfo();
      await this.loadProjects();

      // try to find project and switch to it if URL not pointing to any project
      if (this.$route.path === '/'
        || this.$route.path === '/project'
        || (this.$route.path.startsWith('/project/'))) {
        await this.trySelectMostSuitableProject();
      }

      // display task dialog if query param t specified
      if (this.$route.query.t) {
        const taskId = parseInt(this.$route.query.t || '', 10);
        if (taskId) {
          EventBus.$emit('i-show-task', { taskId });
        }
      }
    },

    async trySelectMostSuitableProject() {
      if (this.projects.length === 0) {
        if (this.$route.path !== '/project/new') {
          await this.$router.push({ path: '/project/new' });
        }
        return;
      }

      let projectId;

      if (this.projectId) {
        projectId = this.projectId;
      }

      if ((projectId == null || !this.projects.some((p) => p.id === projectId))
        && localStorage.getItem('projectId')) {
        projectId = parseInt(localStorage.getItem('projectId'), 10);
      }

      if (projectId == null || !this.projects.some((p) => p.id === projectId)) {
        projectId = this.projects[0].id;
      }

      if (projectId != null) {
        await this.selectProject(projectId);
      }
    },

    async selectProject(projectId) {
      this.userRole = (await axios({
        method: 'get',
        url: `/api/project/${projectId}/role`,
        responseType: 'json',
      })).data;

      localStorage.setItem('projectId', projectId);
      if (this.projectId === projectId) {
        return;
      }

      await this.$router.push({ path: `/project/${projectId}` });
    },

    async loadProjects() {
      this.projects = (await axios({
        method: 'get',
        url: '/api/projects',
        responseType: 'json',
      })).data;
    },

    async loadUserInfo() {
      if (!this.isAuthenticated) {
        return;
      }

      this.user = (await axios({
        method: 'get',
        url: '/api/user',
        responseType: 'json',
      })).data;

      this.systemInfo = (await axios({
        method: 'get',
        url: '/api/info',
        responseType: 'json',
      })).data;
    },

    getProjectColor(projectData) {
      const projectIndex = this.projects.length
        - this.projects.findIndex((p) => p.id === projectData.id);
      return PROJECT_COLORS[projectIndex % PROJECT_COLORS.length];
    },

    getProjectInitials(projectData) {
      const parts = projectData.name.split(/\s/);
      if (parts.length >= 2) {
        return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
      }
      return parts[0].substr(0, 2).toUpperCase();
    },

    async restoreProject() {
      const f = document.createElement('input');
      f.setAttribute('type', 'file');
      f.addEventListener('change', (e) => {
        const file = e.target.files[0];
        if (file) {
          const reader = new FileReader();
          reader.onload = async (ev) => {
            const fileContent = ev.target.result;
            try {
              await axios
                .post('/api/projects/restore', fileContent)
                .then(async (payload) => {
                  this.$router.push({ path: `/project/${payload.data.id}/history` });
                  this.state = 'success';
                  await this.loadProjects();
                });
            } catch (err) {
              EventBus.$emit('i-snackbar', {
                color: 'error',
                text: getErrorMessage(err),
              });
            }
          };
          reader.readAsText(file);
        }
      });
      f.click();
    },

    async signOut() {
      this.snackbar = false;
      this.snackbarColor = '';
      this.snackbarText = '';

      socket.stop();

      (await axios({
        method: 'post',
        url: '/api/auth/logout',
        responseType: 'json',
      }));

      if (this.$route.path !== '/auth/login') {
        await this.$router.push({ path: '/auth/login' });
        this.state = 'success';
      }
    },

    refreshPage() {
      const { location } = document;
      document.location = location;
    },
  },
};
</script>
