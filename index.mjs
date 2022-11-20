import { readFile, writeFile, open, readdir } from "fs/promises";
import { v4 as uuidv4 } from "uuid";
import Parser from "tree-sitter";
import Markdown from "tree-sitter-markdown";
import { format } from 'date-fns'

// console.log(Markdown)

const parser = new Parser();
parser.setLanguage(Markdown.markdown);

let sourceCode = await readFile("./todo.md", "utf-8");
const tree = parser.parse(sourceCode);

let offset = 0;

function editSource(input, position, until) {
  const cutAt = position + offset;
  if (!until) until = 0;
  const first = sourceCode.substring(0, cutAt);
  const second = sourceCode.substring(cutAt + until);
  sourceCode = first + input + second;
  offset += input.length - until;
}

function parseIcsDate(dateString) {
  const date =[
    [0, 4],
    [4, 6],
    [6, 8],
  ]
    .map((idxs) => dateString.substring(...idxs))
    .join("-");

  const time = [
    [9, 11],
    [11, 13],
    [13, 15],
  ]
    .map((idxs) => dateString.substring(...idxs))
    .join(":");

  return new Date(`${date}T${time}`);
}

function createEvent(
  uuid,
  summary,
  date,
  category,
  status = "NEEDS-ACTION",
  comment = ""
) {
  const formattedDate = date.toISOString().replace(/([\-:])|(\.[0-9]+Z)/gi, "");

  return `BEGIN:VCALENDAR
VERSION:2.0
PRODID:crapdav
BEGIN:VTODO
UID:${uuid}
DTSTAMP:${formattedDate}
DUE:${formattedDate}
SUMMARY:${summary.replaceAll("\n", "\\n")}
COMMENT:${comment}
STATUS:${status}
CATEGORIES:${category}
END:VTODO
END:VCALENDAR
`;
}

function parseIcs(ics) {
  let uuid;
  let due;
  let summary;
  let category;
  let status;
  for (const line of ics.split("\n")) {
    const [tag, ...rest] = line.split(":");
    const value = rest.join(":").replaceAll("\r", "");
    if (tag === "UID") uuid = value;
    if (tag === "DUE") due = parseIcsDate(value);
    if (tag === "SUMMARY") summary = value.replaceAll("\\n", "\n");
    if (tag === "CATEGORIES") category = value;
    if (tag === "STATUS") status = value;
  }

  return { uuid, due, summary, category, status };
}

async function parseFile(icsItems) {
  // const todoFile = await open("./todo.md", "r+");

  async function walkSection({ children }) {
    if (!children.length) return;
    const header = children.shift();
    let text = "";
    if (header.type == "atx_heading") {
      text += header.children[1].text.trim();
    }

    const items = [];
    for (const child of children) {
      if (child.type == "list") {
        items.push(await walkList(child, text));
      }
    }
    return items;
  }

  async function walkList(list, category) {
    const items = [];
    for (const { children } of list.children) {
      children.shift();
      const taskStatus = children.shift();
      if (
        taskStatus.type !== "task_list_marker_unchecked" &&
        taskStatus.type !== "task_list_marker_checked"
      ) {
        continue;
      }

      let startsOn;
      let summary = children
        .map((c) => "  " + c.text.trim())
        .join("")
        .replace(
          /[0-9]{4}\-[0-9]{2}\-[0-9]{2}( [0-9]{1,2}:[0-9]{2})?/,
          (match) => {
            if (match.length === 10) {
              match += " 10:00";
            }
            startsOn = new Date(match);
            return "";
          }
        )
        .trim();

      let status =
        taskStatus.type === "task_list_marker_checked"
          ? "COMPLETED"
          : "NEEDS-ACTION";

      let uuid = "";

      const firstChild = children[0];
      const lastChild = children.pop();

      const inlineUuid = summary.match(/#[0-9a-z]{8}$/)?.[0];
      for (const item of icsItems) {
        // console.log()
        if (inlineUuid && item.uuid.substring(0, 8) !== inlineUuid.substring(1))
          continue;
        uuid = item.uuid;

        if (item.status !== status) {
          status = item.status;

          editSource(
            item.status === "COMPLETED" ? "[x]" : "[ ]",
            taskStatus.startIndex,
            3
          );
        }

        if (item.summary !== summary) {
          const tstring = format(item.due, 'yyyy-MM-dd HH:mm');
          const date = tstring.substring(0, 10);
          const time = tstring.substring(11, 16);
          summary = item.summary
          editSource(
            `${date} ${time} ${summary}`,
            firstChild.startIndex,
            lastChild.endIndex - firstChild.startIndex - 1
          );
        }
      }

      if (!uuid) {
        uuid = uuidv4();
        editSource(` #${uuid.substring(0, 8)}`, lastChild.endIndex - 1);
      }

      items.push({
        uuid,
        summary,
        date: startsOn || new Date(),
        category,
        status,
      });
    }
    return items;
  }

  const sections = [];

  for (const child of tree.rootNode.children) {
    if (child.type === "section") sections.push(await walkSection(child));
  }
  return sections.flat(3).filter((d) => d);
}

const icsItems = await Promise.all(
  (
    await readdir("./tests")
  ).map(async (file) => {
    return parseIcs(await readFile(`./tests/${file}`, "utf8"));
  })
);

const items = await parseFile(icsItems);

items.forEach(({ summary, uuid, date, category, status }) => {
  // console.log(summary);
  if (!uuid) throw new Error("No uuid");
  writeFile(
    `./tests/${uuid}.ics`,
    createEvent(uuid, summary, date, category, status),
    "utf8"
  );
});

console.log(sourceCode);

await writeFile("./todo.md", sourceCode, "utf-8");
