# Sipp


Sipp is a lightweight TCP-based server designed to facilitate peer-to-peer communication using a simple protocol, written in [Go](https://go.dev/)

---

### Build
To build, clone the repo:
```
git clone https://github.com/SippChat/Sipp.git
cd sipp
```

Build the server:
```
go build -o sipp-server ./server
```

Run the server:
```
./sipp-server -p 5199
```
By default, the server listens on port 5199. You can specify a different port using the -p flag.

## Configuration
### Straw

Sipp features its own formatting serializer, aptly titled "**Straw**". This serializer is useful for adding color and text formatting to terminal output, and follows a rather rudimentary syntax:

| Tag        | Description          | Example Usage                                   |
|------------|----------------------|-------------------------------------------------|
| `<black>`  | Black text color     | `<black>This is black text</black>`             |
| `<red>`    | Red text color       | `<red>This is red text</red>`                   |
| `<green>`  | Green text color     | `<green>This is green text</green>`             |
| `<yellow>` | Yellow text color    | `<yellow>This is yellow text</yellow>`          |
| `<blue>`   | Blue text color      | `<blue>This is blue text</blue>`                |
| `<magenta>`| Magenta text color   | `<magenta>This is magenta text</magenta>`       |
| `<cyan>`   | Cyan text color      | `<cyan>This is cyan text</cyan>`                |
| `<white>`  | White text color     | `<white>This is white text</white>`             |
| `<b>`      | Bold text            | `<b>This is bold text</b>`                     |
| `<i>`      | Italic text          | `<i>This is italic text</i>`                   |
| `<u>`      | Underline text       | `<u>This is underlined text</u>`                |
| `<s>`      | Strikethrough text   | `<s>This is strikethrough text</s>`             |
| `</tag>`   | Close formatting tag | `</red>`                                        |

#### Notes

- Tags are case-insensitive. For example, `<RED>` and `<red>` will have the same effect.
- Use the closing tag `</tag>` to reset formatting applied by a specific tag.
- Tags may be paired with one another.
---

### MOTD

By default Sipp will hunt for a **MOTD** (Message of the Day) and display that as the resulting message upon the Client - Server Handshake. This can be fully customized using a simple syntax facilitated via [Straw](https://github.com/SippChat/Sipp/blob/main/pkg/straw/straw.go), provided below is a sample MOTD:

```
<green>Welcome to <b>Sipp Server</b>!</green>
<cyan>We're glad to have you here.</cyan>
<yellow>Important:</yellow>
<white>1. Please read the server rules.</white>
<white>2. Follow the community guidelines.</white>
<red>Enjoy your stay!</red>
<blue>For help, type <i>/help</i> in the chat.</blue>
```
---
#### Credits
- [Adventure Library](https://github.com/KyoriPowered/adventure) - The syntax for **Straw** originates (or rather is *inspired*) from the Adventure Library. No code is borrowed or used. 


