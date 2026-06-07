import type { Messages } from '@/i18n';

export const sessionTimelineEnUS: Messages = {
  sessionTimeline: {
    title: 'Conversation',
    empty: 'No conversation yet — send a message to start.',
    thinking: 'Thinking',
    working: 'Thinking…',
    tool: 'Tool',
    result: 'Result',
    resultLines: '{{n}} lines',
    showTerminal: 'Show terminal',
    showChat: 'Show conversation',
    composePlaceholder: 'Message the agent…',
    send: 'Send',
    copy: 'Copy',
    copied: 'Copied',
    moreLines: '+{{n}} more',
    todo: {
      title: 'To-dos',
      done: '{{done}}/{{total}} done',
      pending: 'Pending',
      inProgress: 'In progress',
      completed: 'Completed',
    },
    plan: {
      title: "Claude's plan",
    },
    diff: {
      truncated: 'Diff truncated',
    },
    bash: {
      stderr: 'stderr',
      empty: '(no output)',
    },
    ask: {
      title: 'Your input is needed',
      multiSelectHint: 'Multiple choice',
      singleSelectHint: 'Single choice',
      submit: 'Submit',
      submitted: 'Submitted — waiting for the agent…',
      readonly: 'Answer in the terminal',
    },
  },
};
