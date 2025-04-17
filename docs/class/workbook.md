---
shell: bash
skipPrompts: true
---

# DocServer Interactive Demo

Welcome to the DocServer Interactive Workbook! Think of this as your hands-on lab for learning how the DocServer API works. We'll be using `curl`, a common command-line tool, to send requests to the server and see how it responds. If you have the [Runme VS Code extension](https://runme.dev/) installed, you can run the code examples directly within this document!

**Getting Started: Running the Server**

Before we dive into the API features, we need the DocServer up and running. The very first code block below takes care of this for you. Here's what it does:

1. **Downloads:** It fetches the correct DocServer program (binary) for a Linux system.
2. **Makes it Executable:** It gives your computer permission to run the downloaded program.
3. **Starts the Server:** It launches the DocServer, which will then listen for requests on `http://localhost:8080`.

Go ahead and run that first code block!

```bash { background=true terminalRows=15 }
# Make the working folder
kill $(lsof -t -i :8080) > /dev/null 2>&1 && sleep 2s || true

rm -rf ./demo-sandbox
mkdir -p ./demo-sandbox
cd ./demo-sandbox

# Download the binary (use -f to fail silently if it exists, or remove existing first)
echo "Downloading the binary"
curl -L -o ./docserver-linux-amd64 https://github.com/HWilliams64/docserver/releases/download/v1.0.6/docserver-linux-amd64

# Ensure the file is executable
echo "Making the binary executable"
chmod +x ./docserver-linux-amd64

# Run the server (in the background)
echo "Starting Server..."
./docserver-linux-amd64
```

The server should start listening on `http://localhost:8080` by default.

2. **Tools:** `curl` and `jq` (for easier JSON parsing in the terminal, optional but recommended).

---

## Sign up & Sharing

We will simulate a common document sharing scenario:

1. Sign up Alice.
2. Sign up Bob.
3. Alice logs in.
4. Alice creates a document.
5. Alice shares the document with Bob.
6. Bob logs in.
7. Bob accesses the shared document.
8. Alice (Owner) Edits the Document.
9. Bob Views the Changes Made by Alice.

---

### Step 1: Sign Up Alice

First, let's register a user named Alice.

```bash {"id":"01J0Z8Q5P8G5X7Y4Z3E2A1B0C9"}
export ALICE_EMAIL="alice@example.com"
export ALICE_PASS="password123"
export ALICE_FIRST_NAME="Alice"
export ALICE_LAST_NAME="Wonderland"

curl -s -X POST http://localhost:8080/auth/signup \
 -H "Content-Type: application/json" \
 -d '{
   "email": "'"$ALICE_EMAIL"'",
   "password": "'"$ALICE_PASS"'",
   "first_name": "'"$ALICE_FIRST_NAME"'",
   "last_name": "'"$ALICE_LAST_NAME"'"
 }' | jq .
```

**Output Explanation:**

The server should respond with a `201 Created` status and return the profile details for Alice, including her unique `id`.

<details>
  <summary>🐍 Python</summary>
  <pre><code>
import requests
import json
&#10;
url = "http://localhost:8080/auth/signup"
payload = {
    "email": "test@example.com",
    "password": "secure123",
    "first_name": "Alice",
    "last_name": "Smith"
} 
&#10;
headers = {
    "Content-Type": "application/json"
}
&#10;
response = requests.post(url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
response_obj = response.json()
  </code></pre>
</details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new Gson();&#10;&#10;String url = "http://localhost:8080/auth/signup";&#10;Map<String, String> payload = new HashMap<>();&#10;payload.put("email", "alice.java.gson@example.com");&#10;payload.put("password", "password123");&#10;payload.put("first_name", "Alice");&#10;payload.put("last_name", "JavaGson");&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;&#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  System.out.println(gson.toJson(responseMap));&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>&#10;<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;&#10;val client = OkHttpClient()&#10;val gson = Gson()&#10;&#10;val url = "http://localhost:8080/auth/signup"&#10;val payload = mapOf(&#10;    "email" to "alice.kotlin.gson@example.com",&#10;    "password" to "password123",&#10;    "first_name" to "Alice",&#10;    "last_name" to "KotlinGson"&#10;)&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;&#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        println(gson.toJson(responseMap))&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

---

### Step 2: Sign Up Bob

Now, let's register a second user named Bob.

```bash {"id":"01J0Z8Q5P8H6X8Y5Z4E3A2B1C0"}
export BOB_EMAIL="bob@example.com"
export BOB_PASS="securepass456"
export BOB_FIRST_NAME="Bob"
export BOB_LAST_NAME="The Builder"

curl -s -X POST http://localhost:8080/auth/signup \
 -H "Content-Type: application/json" \
 -d '{
   "email": "'"$BOB_EMAIL"'",
   "password": "'"$BOB_PASS"'",
   "first_name": "'"$BOB_FIRST_NAME"'",
   "last_name": "'"$BOB_LAST_NAME"'"
 }' | jq .
```

**Output Explanation:**

Similar to Alice, the server should respond with `201 Created` and Bob's profile
details, including his `id`.

<details><summary>🐍 Python</summary><pre><code>import requests&#10;import json&#10;&#10;url = "http://localhost:8080/auth/signup"&#10;payload = {&#10;    "email": "bob.python@example.com",&#10;    "password": "securepass456",&#10;    "first_name": "Bob",&#10;    "last_name": "The Builder"&#10;}&#10;&#10;headers = {&#10;    "Content-Type": "application/json"&#10;}&#10;&#10;response = requests.post(url, headers=headers, data=json.dumps(payload))&#10;response.raise_for_status() # Raise an exception for bad status codes&#10;response_obj = response.json()&#10;print(response_obj)</code></pre></details>&#10;<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new Gson();&#10;&#10;String url = "http://localhost:8080/auth/signup";&#10;Map<String, String> payload = new HashMap<>();&#10;payload.put("email", "bob.java.gson@example.com");&#10;payload.put("password", "securepass456");&#10;payload.put("first_name", "Bob");&#10;payload.put("last_name", "JavaGson");&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;&#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  System.out.println(gson.toJson(responseMap));&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>&#10;<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;&#10;val client = OkHttpClient()&#10;val gson = Gson()&#10;&#10;val url = "http://localhost:8080/auth/signup"&#10;val payload = mapOf(&#10;    "email" to "bob.kotlin.gson@example.com",&#10;    "password" to "securepass456",&#10;    "first_name" to "Bob",&#10;    "last_name" to "KotlinGson"&#10;)&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;&#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        println(gson.toJson(responseMap))&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

---

### Step 3: Login Alice

Alice needs to log in to get an authentication token (JWT) for accessing protected endpoints.

```bash {"id":"01J0Z8Q5P8J7X9Y6Z5E4A3B2C1"}
# Note: The token is extracted using jq and exported.
export ALICE_TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
 -H "Content-Type: application/json" \
 -d '{
   "email": "'"$ALICE_EMAIL"'",
   "password": "'"$ALICE_PASS"'"
 }' | jq -r .token)

# Verify the token was captured (optional)
echo "Alice's Token (first 10 chars): ${ALICE_TOKEN:0:10}..."
```

__Output Explanation:__
The server responds with `200 OK` and a JSON object containing the `token`. The command above uses `jq` to extract the token value and exports it as the environment variable `ALICE_TOKEN`. We'll use this token in the `Authorization` header for Alice's subsequent requests.

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_EMAIL and ALICE_PASS are already initialized&#10;
url = "http://localhost:8080/auth/login"
payload = {
    "email": ALICE_EMAIL,
    "password": ALICE_PASS
}&#10;
headers = {
    "Content-Type": "application/json"
}&#10;
response = requests.post(url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
response_obj = response.json()
ALICE_TOKEN = response_obj["token"]&#10;
# Verify the token was captured (optional)
print(f"Alice's Token (first 10 chars): {ALICE_TOKEN[:10]}...")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;&#10;// ALICE_EMAIL and ALICE_PASS are already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new Gson();&#10;&#10;String url = "http://localhost:8080/auth/login";&#10;Map<String, String> payload = new HashMap<>();&#10;payload.put("email", ALICE_EMAIL);&#10;payload.put("password", ALICE_PASS);&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .build();&#10;&#10;String ALICE_TOKEN = "";&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  ALICE_TOKEN = (String) responseMap.get("token");&#10;  &#10;  // Verify the token was captured (optional)&#10;  System.out.println("Alice's Token (first 10 chars): " + &#10;      ALICE_TOKEN.substring(0, Math.min(ALICE_TOKEN.length(), 10)) + "...");&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import kotlin.math.min&#10;&#10;// ALICE_EMAIL and ALICE_PASS are already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = Gson()&#10;&#10;val url = "http://localhost:8080/auth/login"&#10;val payload = mapOf(&#10;    "email" to ALICE_EMAIL,&#10;    "password" to ALICE_PASS&#10;)&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .build()&#10;&#10;var ALICE_TOKEN = ""&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        ALICE_TOKEN = responseMap["token"] as String&#10;        &#10;        // Verify the token was captured (optional)&#10;        println("Alice's Token (first 10 chars): ${ALICE_TOKEN.substring(0, min(ALICE_TOKEN.length, 10))}...")&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

### Step 4: Alice Creates a Document

Now that Alice is logged in (we have her token), she can create a document. The document content can be any valid JSON.

```bash {"id":"01J0Z8Q5P8K8X0Y7Z6E5A4B3C2"}
# Create a simple JSON document
export DOC_ID=$(curl -s -X POST http://localhost:8080/documents \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 -d '{
   "content": {
     "project": "Secret Project X",
     "status": "Draft",
     "notes": "Initial thoughts.",
     "version": 1.0
   }
 }' | jq -r .id)

# Verify the document ID was captured (optional)
echo "Created Document ID: $DOC_ID"
```

**Output Explanation:**

The server responds with `201 Created` and the details of the newly created document, including its unique `id`. We capture this ID into the `DOC_ID` environment variable for later use.

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
payload = {
    "content": {
        "project": "Secret Project X",
        "status": "Draft",
        "notes": "Initial thoughts.",
        "version": 1.0
    }
}&#10;
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.post(url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
response_obj = response.json()
DOC_ID = response_obj["id"]&#10;
# Verify the document ID was captured (optional)
print(f"Created Document ID: {DOC_ID}")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new Gson();&#10;&#10;String url = "http://localhost:8080/documents";&#10;&#10;// Create the content object&#10;Map<String, Object> content = new HashMap<>();&#10;content.put("project", "Secret Project X");&#10;content.put("status", "Draft");&#10;content.put("notes", "Initial thoughts.");&#10;content.put("version", 1.0);&#10;&#10;// Create the payload with content nested inside&#10;Map<String, Object> payload = new HashMap<>();&#10;payload.put("content", content);&#10;&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;String DOC_ID = "";&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  DOC_ID = (String) responseMap.get("id");&#10;  &#10;  // Verify the document ID was captured (optional)&#10;  System.out.println("Created Document ID: " + DOC_ID);&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = Gson()&#10;&#10;val url = "http://localhost:8080/documents"&#10;&#10;// Create the content object&#10;val content = mapOf(&#10;    "project" to "Secret Project X",&#10;    "status" to "Draft",&#10;    "notes" to "Initial thoughts.",&#10;    "version" to 1.0&#10;)&#10;&#10;// Create the payload with content nested inside&#10;val payload = mapOf(&#10;    "content" to content&#10;)&#10;&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;var DOC_ID = ""&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        DOC_ID = responseMap["id"] as String&#10;        &#10;        // Verify the document ID was captured (optional)&#10;        println("Created Document ID: $DOC_ID")&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

---

### Step 5: Alice Shares the Document with Bob

To share the document, Alice needs Bob's `id`. First, Alice searches for Bob using his email address. Then, she uses the document ID and Bob's ID to grant access.

```bash {"id":"01J0Z8Q5P8M9X1Y8Z7E6A5B4C3"}
# 1. Find Bob's ID (Alice needs to be logged in)
export BOB_ID=$(curl -s -X GET "http://localhost:8080/profiles?email=$BOB_EMAIL" \
 -H "Authorization: Bearer $ALICE_TOKEN" | jq -r '.data[0].id') # Assumes email is unique and gets ID from first result in data array

echo "Found Bob's ID: $BOB_ID"

# 2. Share the document with Bob (Alice must own the document)
curl -s -X PUT "http://localhost:8080/documents/$DOC_ID/shares/$BOB_ID" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 -H "Content-Type: application/json" \
 -d '{
   "permission": "edit"
 }' | jq .
```

**Output Explanation:**

1. The first `curl` command searches for profiles matching Bob's email. We extract the `id` from the first result (assuming emails are unique) and store it in `BOB_ID`.
2. The second `curl` command adds Bob to the document's share list. The server should respond with `204 No Content` on success. (Note: The API currently doesn't support different permission levels like "edit" vs "view" through this endpoint).

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# BOB_EMAIL, ALICE_TOKEN, and DOC_ID are already initialized&#10;
# Step 1: Find Bob's ID by searching for his email
search_url = f"http://localhost:8080/profiles?email={BOB_EMAIL}"
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.get(search_url, headers=headers)
response.raise_for_status()
response_obj = response.json()
BOB_ID = response_obj["data"][0]["id"]  # Assumes email is unique&#10;
print(f"Found Bob's ID: {BOB_ID}")&#10;
# Step 2: Share the document with Bob
share_url = f"http://localhost:8080/documents/{DOC_ID}/shares/{BOB_ID}"
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {ALICE_TOKEN}"
}
payload = {
    "permission": "edit"
}&#10;
response = requests.put(share_url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
print(f"Document shared with Bob: {response.status_code}")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.reflect.TypeToken;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;import java.util.List;&#10;&#10;// BOB_EMAIL, ALICE_TOKEN, and DOC_ID are already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new Gson();&#10;&#10;// Step 1: Find Bob's ID by searching for his email&#10;String searchUrl = "http://localhost:8080/profiles?email=" + BOB_EMAIL;&#10;&#10;Request searchRequest = new Request.Builder()&#10;    .url(searchUrl)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;String BOB_ID = "";&#10;try (Response response = client.newCall(searchRequest).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map<String, Object> responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  List<Map<String, Object>> dataList = (List<Map<String, Object>>) responseMap.get("data");&#10;  BOB_ID = (String) dataList.get(0).get("id");  // Assumes email is unique&#10;  &#10;  System.out.println("Found Bob's ID: " + BOB_ID);&#10;&#10;  // Step 2: Share the document with Bob&#10;  String shareUrl = "http://localhost:8080/documents/" + DOC_ID + "/shares/" + BOB_ID;&#10;  &#10;  // Create the permission payload&#10;  Map<String, String> payload = new HashMap<>();&#10;  payload.put("permission", "edit");&#10;  String json = gson.toJson(payload);&#10;&#10;  MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;  RequestBody body = RequestBody.create(json, JSON);&#10;&#10;  Request shareRequest = new Request.Builder()&#10;      .url(shareUrl)&#10;      .put(body)&#10;      .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;      .build();&#10;&#10;  try (Response shareResponse = client.newCall(shareRequest).execute()) {&#10;    if (!shareResponse.isSuccessful()) throw new IOException("Unexpected code " + shareResponse);&#10;    System.out.println("Document shared with Bob: " + shareResponse.code());&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;&#10;// BOB_EMAIL, ALICE_TOKEN, and DOC_ID are already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = Gson()&#10;&#10;// Step 1: Find Bob's ID by searching for his email&#10;val searchUrl = "http://localhost:8080/profiles?email=$BOB_EMAIL"&#10;&#10;val searchRequest = Request.Builder()&#10;    .url(searchUrl)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;var BOB_ID = ""&#10;try {&#10;    client.newCall(searchRequest).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        val dataList = responseMap["data"] as List<Map<String, Any>>&#10;        BOB_ID = dataList[0]["id"] as String  // Assumes email is unique&#10;        &#10;        println("Found Bob's ID: $BOB_ID")&#10;&#10;        // Step 2: Share the document with Bob&#10;        val shareUrl = "http://localhost:8080/documents/$DOC_ID/shares/$BOB_ID"&#10;        &#10;        // Create the permission payload&#10;        val payload = mapOf(&#10;            "permission" to "edit"&#10;        )&#10;        val json = gson.toJson(payload)&#10;&#10;        val mediaType = "application/json; charset=utf-8".toMediaType()&#10;        val body = json.toRequestBody(mediaType)&#10;&#10;        val shareRequest = Request.Builder()&#10;            .url(shareUrl)&#10;            .put(body)&#10;            .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;            .build()&#10;&#10;        client.newCall(shareRequest).execute().use { shareResponse ->&#10;            if (!shareResponse.isSuccessful) throw IOException("Unexpected code $shareResponse")&#10;            println("Document shared with Bob: ${shareResponse.code}")&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

---

### Step 6: Login Bob

Now, Bob logs in to get his own authentication token.

```bash {"id":"01J0Z8Q5P8N0X2Y9Z8E7A6B5C4"}
export BOB_TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
 -H "Content-Type: application/json" \
 -d '{
   "email": "'"$BOB_EMAIL"'",
   "password": "'"$BOB_PASS"'"
 }' | jq -r .token)

# Verify the token was captured (optional)
echo "Bob's Token (first 10 chars): ${BOB_TOKEN:0:10}..."
```

**Output Explanation:**

Similar to Alice's login, Bob receives a `200 OK` response with his JWT, which is stored in the `BOB_TOKEN` environment variable.

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# BOB_EMAIL and BOB_PASS are already initialized&#10;
url = "http://localhost:8080/auth/login"
payload = {
    "email": BOB_EMAIL,
    "password": BOB_PASS
}&#10;
headers = {
    "Content-Type": "application/json"
}&#10;
response = requests.post(url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
response_obj = response.json()
BOB_TOKEN = response_obj["token"]&#10;
# Verify the token was captured (optional)
print(f"Bob's Token (first 10 chars): {BOB_TOKEN[:10]}...")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;&#10;// BOB_EMAIL and BOB_PASS are already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new Gson();&#10;&#10;String url = "http://localhost:8080/auth/login";&#10;Map<String, String> payload = new HashMap<>();&#10;payload.put("email", BOB_EMAIL);&#10;payload.put("password", BOB_PASS);&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .build();&#10;&#10;String BOB_TOKEN = "";&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  BOB_TOKEN = (String) responseMap.get("token");&#10;  &#10;  // Verify the token was captured (optional)&#10;  System.out.println("Bob's Token (first 10 chars): " + &#10;      BOB_TOKEN.substring(0, Math.min(BOB_TOKEN.length(), 10)) + "...");&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import kotlin.math.min&#10;&#10;// BOB_EMAIL and BOB_PASS are already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = Gson()&#10;&#10;val url = "http://localhost:8080/auth/login"&#10;val payload = mapOf(&#10;    "email" to BOB_EMAIL,&#10;    "password" to BOB_PASS&#10;)&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .build()&#10;&#10;var BOB_TOKEN = ""&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        BOB_TOKEN = responseMap["token"] as String&#10;        &#10;        // Verify the token was captured (optional)&#10;        println("Bob's Token (first 10 chars): ${BOB_TOKEN.substring(0, min(BOB_TOKEN.length, 10))}...")&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

---

### Step 7: Bob Accesses the Shared Document

Bob can now use his token to access the document Alice shared with him.

```bash {"id":"01J0Z8Q5P8P1X3Y0Z9E8A7B6C5"}
curl -s -X GET "http://localhost:8080/documents/$DOC_ID" \
 -H "Authorization: Bearer $BOB_TOKEN" | jq .
```

**Output Explanation:**

The server responds with `200 OK` and the full document content as created by Alice. This confirms Bob has read access.

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# BOB_TOKEN and DOC_ID are already initialized&#10;
url = f"http://localhost:8080/documents/{DOC_ID}"
headers = {
    "Authorization": f"Bearer {BOB_TOKEN}"
}&#10;
response = requests.get(url, headers=headers)
response.raise_for_status()
document = response.json()&#10;
# Print document content
print(json.dumps(document, indent=2))
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;&#10;// BOB_TOKEN and DOC_ID are already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;String url = "http://localhost:8080/documents/" + DOC_ID;&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + BOB_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map documentMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print document content&#10;  System.out.println(gson.toJson(documentMap));&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// BOB_TOKEN and DOC_ID are already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;val url = "http://localhost:8080/documents/$DOC_ID"&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $BOB_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val documentMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print document content&#10;        println(gson.toJson(documentMap))&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

---

### Step 8: Alice (Owner) Edits the Document

Only the owner can edit the document content. Alice will update the document she created earlier.

```bash {"id":"01J0Z8Q5P8Q2X4Y1Z0E9A8B7C6"}
curl -s -X PUT "http://localhost:8080/documents/$DOC_ID" \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 -d '{
  "content": {
    "project": "Secret Project X",
    "status": "In Review",
    "notes": "Initial thoughts, plus Alice added updates.",
    "version": 1.1,
    "updated_by": "Alice"
   }
 }' | jq .
```

**Output Explanation:**

The server responds with `200 OK` and the updated document content. Note that Alice changed the `status`, `notes`, `version`, and added an `updated_by` field.

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN and DOC_ID are already initialized&#10;
url = f"http://localhost:8080/documents/{DOC_ID}"
payload = {
    "content": {
        "project": "Secret Project X",
        "status": "In Review",
        "notes": "Initial thoughts, plus Alice added updates.",
        "version": 1.1,
        "updated_by": "Alice"
    }
}&#10;
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.put(url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
updated_document = response.json()&#10;
# Print updated document content
print(json.dumps(updated_document, indent=2))
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;&#10;// ALICE_TOKEN and DOC_ID are already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;String url = "http://localhost:8080/documents/" + DOC_ID;&#10;&#10;// Create the content object&#10;Map<String, Object> content = new HashMap<>();&#10;content.put("project", "Secret Project X");&#10;content.put("status", "In Review");&#10;content.put("notes", "Initial thoughts, plus Alice added updates.");&#10;content.put("version", 1.1);&#10;content.put("updated_by", "Alice");&#10;&#10;// Create the payload with content nested inside&#10;Map<String, Object> payload = new HashMap<>();&#10;payload.put("content", content);&#10;&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .put(body)&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map documentMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print updated document content&#10;  System.out.println(gson.toJson(documentMap));&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN and DOC_ID are already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;val url = "http://localhost:8080/documents/$DOC_ID"&#10;&#10;// Create the content object&#10;val content = mapOf(&#10;    "project" to "Secret Project X",&#10;    "status" to "In Review",&#10;    "notes" to "Initial thoughts, plus Alice added updates.",&#10;    "version" to 1.1,&#10;    "updated_by" to "Alice"&#10;)&#10;&#10;// Create the payload with content nested inside&#10;val payload = mapOf(&#10;    "content" to content&#10;)&#10;&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .put(body)&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val documentMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print updated document content&#10;        println(gson.toJson(documentMap))&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

---

### Step 9: Bob Views the Changes Made by Alice

Finally, Bob uses his token to view the document again and see the changes Alice made.

```bash {"id":"01J0Z8Q5P8R3X5Y2Z1F0A9B8C7"}
curl -s -X GET "http://localhost:8080/documents/$DOC_ID" \
 -H "Authorization: Bearer $BOB_TOKEN" | jq .
```

**Output Explanation:**

The server responds with `200 OK` and the document content, now reflecting the modifications made by Alice in the previous step. This demonstrates that Bob can view the changes made by the owner to the shared document.

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# BOB_TOKEN and DOC_ID are already initialized&#10;
url = f"http://localhost:8080/documents/{DOC_ID}"
headers = {
    "Authorization": f"Bearer {BOB_TOKEN}"
}&#10;
response = requests.get(url, headers=headers)
response.raise_for_status()
document = response.json()&#10;
# Print document content to see Alice's changes
print(json.dumps(document, indent=2))&#10;
# Verify the updated fields
content = document.get("content", {})
print(f"Status: {content.get('status')}")
print(f"Version: {content.get('version')}")
print(f"Updated by: {content.get('updated_by')}")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;&#10;// BOB_TOKEN and DOC_ID are already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;String url = "http://localhost:8080/documents/" + DOC_ID;&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + BOB_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map documentMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print document content to see Alice's changes&#10;  System.out.println(gson.toJson(documentMap));&#10;  &#10;  // Verify the updated fields&#10;  Map<String, Object> content = (Map<String, Object>) documentMap.get("content");&#10;  System.out.println("Status: " + content.get("status"));&#10;  System.out.println("Version: " + content.get("version"));&#10;  System.out.println("Updated by: " + content.get("updated_by"));&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// BOB_TOKEN and DOC_ID are already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;val url = "http://localhost:8080/documents/$DOC_ID"&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $BOB_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val documentMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print document content to see Alice's changes&#10;        println(gson.toJson(documentMap))&#10;        &#10;        // Verify the updated fields&#10;        val content = documentMap["content"] as Map<*, *>&#10;        println("Status: ${content["status"]}")&#10;        println("Version: ${content["version"]}")&#10;        println("Updated by: ${content["updated_by"]}")&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

---

This concludes the basic demonstration of the DocServer API workflow. You can explore other endpoints like deleting documents, removing shares, updating profiles, and using content queries via the [Swagger UI documentation](http://localhost:8080/swagger/index.html) (available when the server is running).

---

## Document Content Querying

This section demonstrates how to use the `content_query` parameter with the `GET /documents` endpoint to search for documents based on the data _inside_ their JSON `content`.

**Assumptions:**

* The DocServer is still running from the previous steps.
* Alice is logged in, and her token is stored in the `ALICE_TOKEN` environment variable.

### Step 10: Create Example Documents for Querying

Let's have Alice create a few documents with different structures and data types.

**Document 1: Simple Report**

```bash {"id":"01J10B0F5E1G2H3J4K5L6M7N8P"}
# Create a simple report document
curl -s -X POST http://localhost:8080/documents \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 -d '{
   "content": {
     "type": "report",
     "year": 2024,
     "status": "final",
     "approved": true,
     "title": "Q1 Performance Review"
   }
 }' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
payload = {
    "content": {
        "type": "report",
        "year": 2024,
        "status": "final",
        "approved": True,
        "title": "Q1 Performance Review"
    }
}&#10;
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.post(url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
document = response.json()&#10;
# Print document content
print(json.dumps(document, indent=2))
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;String url = "http://localhost:8080/documents";&#10;&#10;// Create the content object&#10;Map<String, Object> content = new HashMap<>();&#10;content.put("type", "report");&#10;content.put("year", 2024);&#10;content.put("status", "final");&#10;content.put("approved", true);&#10;content.put("title", "Q1 Performance Review");&#10;&#10;// Create the payload with content nested inside&#10;Map<String, Object> payload = new HashMap<>();&#10;payload.put("content", content);&#10;&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map documentMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print document content&#10;  System.out.println(gson.toJson(documentMap));&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;val url = "http://localhost:8080/documents"&#10;&#10;// Create the content object&#10;val content = mapOf(&#10;    "type" to "report",&#10;    "year" to 2024,&#10;    "status" to "final",&#10;    "approved" to true,&#10;    "title" to "Q1 Performance Review"&#10;)&#10;&#10;// Create the payload with content nested inside&#10;val payload = mapOf(&#10;    "content" to content&#10;)&#10;&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val documentMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print document content&#10;        println(gson.toJson(documentMap))&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Document 2: Task with Nested Object and Array**

```bash {"id":"01J10B0F5E9G8H7J6K5L4M3N2P"}
# Create a task document
curl -s -X POST http://localhost:8080/documents \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 -d '{
   "content": {
     "type": "task",
     "project": {
       "name": "Project Phoenix",
       "priority": 1
     },
     "assignee": "Alice Wonderland",
     "tags": ["backend", "urgent", "api"],
     "effort_hours": 8
   }
 }' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
payload = {
    "content": {
        "type": "task",
        "project": {
            "name": "Project Phoenix",
            "priority": 1
        },
        "assignee": "Alice Wonderland",
        "tags": ["backend", "urgent", "api"],
        "effort_hours": 8
    }
}&#10;
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.post(url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
document = response.json()&#10;
# Print document content
print(json.dumps(document, indent=2))
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;import java.util.Arrays;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;String url = "http://localhost:8080/documents";&#10;&#10;// Create the project nested object&#10;Map<String, Object> project = new HashMap<>();&#10;project.put("name", "Project Phoenix");&#10;project.put("priority", 1);&#10;&#10;// Create the tags array&#10;List<String> tags = Arrays.asList("backend", "urgent", "api");&#10;&#10;// Create the content object&#10;Map<String, Object> content = new HashMap<>();&#10;content.put("type", "task");&#10;content.put("project", project);&#10;content.put("assignee", "Alice Wonderland");&#10;content.put("tags", tags);&#10;content.put("effort_hours", 8);&#10;&#10;// Create the payload with content nested inside&#10;Map<String, Object> payload = new HashMap<>();&#10;payload.put("content", content);&#10;&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map documentMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print document content&#10;  System.out.println(gson.toJson(documentMap));&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;val url = "http://localhost:8080/documents"&#10;&#10;// Create the project nested object&#10;val project = mapOf(&#10;    "name" to "Project Phoenix",&#10;    "priority" to 1&#10;)&#10;&#10;// Create the content object with nested project and tags array&#10;val content = mapOf(&#10;    "type" to "task",&#10;    "project" to project,&#10;    "assignee" to "Alice Wonderland",&#10;    "tags" to listOf("backend", "urgent", "api"),&#10;    "effort_hours" to 8&#10;)&#10;&#10;// Create the payload with content nested inside&#10;val payload = mapOf(&#10;    "content" to content&#10;)&#10;&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val documentMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print document content&#10;        println(gson.toJson(documentMap))&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Document 3: Meeting Notes with Nested Arrays/Objects**

```bash {"id":"01J10B0F5F0G1H2J3K4L5M6N7Q"}
# Create meeting notes document
curl -s -X POST http://localhost:8080/documents \
 -H "Content-Type: application/json" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 -d '{
   "content": {
     "type": "meeting_notes",
     "date": "2024-04-15",
     "attendees": [
       {"name": "Alice Wonderland", "role": "lead"},
       {"name": "Bob The Builder", "role": "dev"},
       {"name": "Charlie Chaplin", "role": "qa"}
     ],
     "action_items": [
       {"task": "Update API docs", "owner": "Alice Wonderland", "due": "2024-04-20", "completed": false},
       {"task": "Write unit tests", "owner": "Bob The Builder", "due": "2024-04-22", "completed": null}
     ],
     "summary": "Discussed project milestones and blockers."
   }
 }' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
payload = {
    "content": {
        "type": "meeting_notes",
        "date": "2024-04-15",
        "attendees": [
            {"name": "Alice Wonderland", "role": "lead"},
            {"name": "Bob The Builder", "role": "dev"},
            {"name": "Charlie Chaplin", "role": "qa"}
        ],
        "action_items": [
            {"task": "Update API docs", "owner": "Alice Wonderland", "due": "2024-04-20", "completed": False},
            {"task": "Write unit tests", "owner": "Bob The Builder", "due": "2024-04-22", "completed": None}
        ],
        "summary": "Discussed project milestones and blockers."
    }
}&#10;
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.post(url, headers=headers, data=json.dumps(payload))
response.raise_for_status()
document = response.json()&#10;
# Print document content
print(json.dumps(document, indent=2))
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.HashMap;&#10;import java.util.ArrayList;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;String url = "http://localhost:8080/documents";&#10;&#10;// Create attendees array with nested objects&#10;List<Map<String, String>> attendees = new ArrayList<>();&#10;&#10;Map<String, String> attendee1 = new HashMap<>();&#10;attendee1.put("name", "Alice Wonderland");&#10;attendee1.put("role", "lead");&#10;attendees.add(attendee1);&#10;&#10;Map<String, String> attendee2 = new HashMap<>();&#10;attendee2.put("name", "Bob The Builder");&#10;attendee2.put("role", "dev");&#10;attendees.add(attendee2);&#10;&#10;Map<String, String> attendee3 = new HashMap<>();&#10;attendee3.put("name", "Charlie Chaplin");&#10;attendee3.put("role", "qa");&#10;attendees.add(attendee3);&#10;&#10;// Create action_items array with nested objects&#10;List<Map<String, Object>> actionItems = new ArrayList<>();&#10;&#10;Map<String, Object> item1 = new HashMap<>();&#10;item1.put("task", "Update API docs");&#10;item1.put("owner", "Alice Wonderland");&#10;item1.put("due", "2024-04-20");&#10;item1.put("completed", false);&#10;actionItems.add(item1);&#10;&#10;Map<String, Object> item2 = new HashMap<>();&#10;item2.put("task", "Write unit tests");&#10;item2.put("owner", "Bob The Builder");&#10;item2.put("due", "2024-04-22");&#10;item2.put("completed", null);&#10;actionItems.add(item2);&#10;&#10;// Create the content object&#10;Map<String, Object> content = new HashMap<>();&#10;content.put("type", "meeting_notes");&#10;content.put("date", "2024-04-15");&#10;content.put("attendees", attendees);&#10;content.put("action_items", actionItems);&#10;content.put("summary", "Discussed project milestones and blockers.");&#10;&#10;// Create the payload with content nested inside&#10;Map<String, Object> payload = new HashMap<>();&#10;payload.put("content", content);&#10;&#10;String json = gson.toJson(payload);&#10;&#10;MediaType JSON = MediaType.parse("application/json; charset=utf-8");&#10;RequestBody body = RequestBody.create(json, JSON);&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map documentMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print document content&#10;  System.out.println(gson.toJson(documentMap));&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import okhttp3.MediaType.Companion.toMediaType&#10;import okhttp3.RequestBody.Companion.toRequestBody&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;val url = "http://localhost:8080/documents"&#10;&#10;// Create attendees list with nested objects&#10;val attendees = listOf(&#10;    mapOf("name" to "Alice Wonderland", "role" to "lead"),&#10;    mapOf("name" to "Bob The Builder", "role" to "dev"),&#10;    mapOf("name" to "Charlie Chaplin", "role" to "qa")&#10;)&#10;&#10;// Create action_items list with nested objects&#10;val actionItems = listOf(&#10;    mapOf(&#10;        "task" to "Update API docs",&#10;        "owner" to "Alice Wonderland",&#10;        "due" to "2024-04-20",&#10;        "completed" to false&#10;    ),&#10;    mapOf(&#10;        "task" to "Write unit tests",&#10;        "owner" to "Bob The Builder",&#10;        "due" to "2024-04-22",&#10;        "completed" to null&#10;    )&#10;)&#10;&#10;// Create the content object&#10;val content = mapOf(&#10;    "type" to "meeting_notes",&#10;    "date" to "2024-04-15",&#10;    "attendees" to attendees,&#10;    "action_items" to actionItems,&#10;    "summary" to "Discussed project milestones and blockers."&#10;)&#10;&#10;// Create the payload with content nested inside&#10;val payload = mapOf(&#10;    "content" to content&#10;)&#10;&#10;val json = gson.toJson(payload)&#10;&#10;val mediaType = "application/json; charset=utf-8".toMediaType()&#10;val body = json.toRequestBody(mediaType)&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .post(body)&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val documentMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print document content&#10;        println(gson.toJson(documentMap))&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

Each command should return a `201 Created` status and the details of the newly created document. We now have three documents owned by Alice with varied content to query against.

### Understanding the Query Syntax

The `GET /documents` endpoint accepts `content_query` parameters for conditions, interleaved with explicit logical operators (`and` or `or`). Each condition query follows the format:

`path operator value`

* **`path`**: A path to navigate the JSON `content` using dot notation for objects (e.g., `project.name`) and numeric indices for arrays (e.g., `attendees.0.name`). If omitted, the query applies to the root of the `content`.
* **`operator`**: How to compare the value found at the `path` with the provided `value`. Common operators include:

   * `equals`, `notequals` - For strings, numbers, booleans, null.
   * `greaterthan`, `greaterthanorequals`, `lessthan`, `lessthanorequals` - For numbers.
   * `contains`, `startswith`, `endswith` - For strings (case-sensitive by default). Add `-insensitive` suffix (e.g., `equals-insensitive`, `contains-insensitive`) for case-insensitive matching. `contains` also works for checking if an *array* contains a specific primitive value (string, number, boolean, null).

* **`value`**: The value to compare against.

   * **Strings:** MUST be enclosed in double quotes (e.g., `"final"`, `"Project Phoenix"`).
   * **Numbers:** Use directly (e.g., `2024`, `1`).
   * **Booleans:** Use `true` or `false` directly.
   * **Null:** Use `null` directly.

**Important: URL Encoding**

When using `curl` (especially from the command line), you need to make sure the query parameters are correctly URL-encoded. This is particularly important for string values containing spaces or special characters, and for the double quotes around string values. The `-G` flag combined with `--data-urlencode` in `curl` helps manage this.

### Step 11: Basic Query Examples

Let's try some queries on the documents Alice created.

**Example 1: Find all reports (Top-level string `equals`)**

We want documents where the `type` field inside `content` is exactly `"report"`.

```bash {"id":"01J10B0F5F8G9H0J1K2L3M4N5R","terminalRows":"22"}
# Query: content.type equals "report"
curl -G -s "http://localhost:8080/documents" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 --data-urlencode 'content_query=type equals "report"' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
params = {
    "content_query": 'type equals "report"'
}
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.get(url, headers=headers, params=params)
response.raise_for_status()
result = response.json()&#10;
# Print matching documents
print(json.dumps(result, indent=2))&#10;
# Verify we found the expected report
if result.get("data") and len(result["data"]) > 0:
    doc = result["data"][0]
    content = doc.get("content", {})
    print(f"Found document with title: {content.get('title')}")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;HttpUrl url = HttpUrl.parse("http://localhost:8080/documents")&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "type equals \"report\"")&#10;    .build();&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print matching documents&#10;  System.out.println(gson.toJson(responseMap));&#10;  &#10;  // Verify we found the expected report&#10;  List<Map<String, Object>> data = (List<Map<String, Object>>) responseMap.get("data");&#10;  if (data != null && !data.isEmpty()) {&#10;    Map<String, Object> doc = data.get(0);&#10;    Map<String, Object> content = (Map<String, Object>) doc.get("content");&#10;    System.out.println("Found document with title: " + content.get("title"));&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;val url = HttpUrl.parse("http://localhost:8080/documents")!!&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "type equals \"report\"")&#10;    .build()&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print matching documents&#10;        println(gson.toJson(responseMap))&#10;        &#10;        // Verify we found the expected report&#10;        val data = responseMap["data"] as? List<Map<String, Any>>&#10;        if (data != null && data.isNotEmpty()) {&#10;            val doc = data[0]&#10;            val content = doc["content"] as? Map<*, *>&#10;            println("Found document with title: ${content?.get("title")}")&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

This should return a JSON response containing the "Q1 Performance Review" document (Document 1) in the `data` array, as its `type` is "report".

**Example 2: Find items from 2024 or later (Top-level number `greaterthanorequals`)**

We want documents where the `year` field is greater than or equal to `2024`.

```bash {"id":"01J10B0F5G0G1H2J3K4L5M6N7S","terminalRows":"21"}
# Query: content.year greater than or equal to 2024
curl -G -s "http://localhost:8080/documents" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 --data-urlencode 'content_query=year greaterthanorequals 2024' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
params = {
    "content_query": 'year greaterthanorequals 2024'
}
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.get(url, headers=headers, params=params)
response.raise_for_status()
result = response.json()&#10;
# Print matching documents
print(json.dumps(result, indent=2))&#10;
# Verify we found documents from 2024 or later
if result.get("data") and len(result["data"]) > 0:
    for doc in result["data"]:
        content = doc.get("content", {})
        if "year" in content:
            print(f"Document year: {content.get('year')}")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;HttpUrl url = HttpUrl.parse("http://localhost:8080/documents")&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "year greaterthanorequals 2024")&#10;    .build();&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print matching documents&#10;  System.out.println(gson.toJson(responseMap));&#10;  &#10;  // Verify we found documents from 2024 or later&#10;  List<Map<String, Object>> data = (List<Map<String, Object>>) responseMap.get("data");&#10;  if (data != null && !data.isEmpty()) {&#10;    for (Map<String, Object> doc : data) {&#10;      Map<String, Object> content = (Map<String, Object>) doc.get("content");&#10;      if (content.containsKey("year")) {&#10;        System.out.println("Document year: " + content.get("year"));&#10;      }&#10;    }&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;val url = HttpUrl.parse("http://localhost:8080/documents")!!&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "year greaterthanorequals 2024")&#10;    .build()&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print matching documents&#10;        println(gson.toJson(responseMap))&#10;        &#10;        // Verify we found documents from 2024 or later&#10;        val data = responseMap["data"] as? List<Map<String, Any>>&#10;        if (data != null && data.isNotEmpty()) {&#10;            data.forEach { doc ->&#10;                val content = doc["content"] as? Map<*, *>&#10;                if (content?.containsKey("year") == true) {&#10;                    println("Document year: ${content["year"]}")&#10;                }&#10;            }&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

This should return only Document 1 ("Q1 Performance Review"), as it's the only one with a `year` field meeting the criteria.

**Example 3: Find approved reports (Explicit `AND`)**

We want documents where `type` is `"report"` AND `approved` is `true`. We provide the two conditions separated by `content_query=and`.

```bash {"id":"01J10B0F5G8G9H0J1K2L3M4N5T","terminalRows":"21"}
# Query: content.type equals "report" AND content.approved equals true
curl -G -s "http://localhost:8080/documents" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 --data-urlencode 'content_query=type equals "report"' \
 --data-urlencode 'content_query=and' \
 --data-urlencode 'content_query=approved equals true' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
# We need to send content_query multiple times, so we use a list of tuples
params = [
    ('content_query', 'type equals "report"'),
    ('content_query', 'and'),
    ('content_query', 'approved equals true')
]
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
# Pass params as a list to requests.get
response = requests.get(url, headers=headers, params=params)
response.raise_for_status()
result = response.json()&#10;
# Print matching documents
print(json.dumps(result, indent=2))&#10;
# Verify we found approved reports
if result.get("data") and len(result["data"]) > 0:
    for doc in result["data"]:
        content = doc.get("content", {})
        print(f"Document type: {content.get('type')}, Approved: {content.get('approved')}")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;HttpUrl url = HttpUrl.parse("http://localhost:8080/documents")&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "type equals \"report\"")&#10;    .addQueryParameter("content_query", "and")&#10;    .addQueryParameter("content_query", "approved equals true")&#10;    .build();&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print matching documents&#10;  System.out.println(gson.toJson(responseMap));&#10;  &#10;  // Verify we found approved reports&#10;  List<Map<String, Object>> data = (List<Map<String, Object>>) responseMap.get("data");&#10;  if (data != null && !data.isEmpty()) {&#10;    for (Map<String, Object> doc : data) {&#10;      Map<String, Object> content = (Map<String, Object>) doc.get("content");&#10;      System.out.println("Document type: " + content.get("type") + ", Approved: " + content.get("approved"));&#10;    }&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;val url = HttpUrl.parse("http://localhost:8080/documents")!!&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "type equals \"report\"")&#10;    .addQueryParameter("content_query", "and")&#10;    .addQueryParameter("content_query", "approved equals true")&#10;    .build()&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print matching documents&#10;        println(gson.toJson(responseMap))&#10;        &#10;        // Verify we found approved reports&#10;        val data = responseMap["data"] as? List<Map<String, Any>>&#10;        if (data != null && data.isNotEmpty()) {&#10;            data.forEach { doc ->&#10;                val content = doc["content"] as? Map<*, *>&#10;                println("Document type: ${content?.get("type")}, Approved: ${content?.get("approved")}")&#10;            }&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

This should again return only Document 1, as it matches both conditions.

### Step 12: Nested and Array Query Examples

Let's query inside nested structures.

**Example 4: Find tasks for "Project Phoenix" (Nested object string `equals`)**

We need to use dot notation to access `name` inside the `project` object.

```bash {"id":"01J10B0F5H0G1H2J3K4L5M6N7U"}
# Query: content.project.name equals "Project Phoenix"
curl -G -s "http://localhost:8080/documents" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 --data-urlencode 'content_query=project.name equals "Project Phoenix"' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
params = {
    "content_query": 'project.name equals "Project Phoenix"'
}
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.get(url, headers=headers, params=params)
response.raise_for_status()
result = response.json()&#10;
# Print matching documents
print(json.dumps(result, indent=2))&#10;
# Verify we found the task for Project Phoenix
if result.get("data") and len(result["data"]) > 0:
    for doc in result["data"]:
        content = doc.get("content", {})
        project = content.get("project", {})
        print(f"Document project name: {project.get('name')}")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;HttpUrl url = HttpUrl.parse("http://localhost:8080/documents")&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "project.name equals \"Project Phoenix\"")&#10;    .build();&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print matching documents&#10;  System.out.println(gson.toJson(responseMap));&#10;  &#10;  // Verify we found the task for Project Phoenix&#10;  List<Map<String, Object>> data = (List<Map<String, Object>>) responseMap.get("data");&#10;  if (data != null && !data.isEmpty()) {&#10;    for (Map<String, Object> doc : data) {&#10;      Map<String, Object> content = (Map<String, Object>) doc.get("content");&#10;      Map<String, Object> project = (Map<String, Object>) content.get("project");&#10;      if (project != null) {&#10;        System.out.println("Document project name: " + project.get("name"));&#10;      }&#10;    }&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;val url = HttpUrl.parse("http://localhost:8080/documents")!!&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "project.name equals \"Project Phoenix\"")&#10;    .build()&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print matching documents&#10;        println(gson.toJson(responseMap))&#10;        &#10;        // Verify we found the task for Project Phoenix&#10;        val data = responseMap["data"] as? List<Map<String, Any>>&#10;        if (data != null && data.isNotEmpty()) {&#10;            data.forEach { doc ->&#10;                val content = doc["content"] as? Map<*, *>&#10;                val project = content?.get("project") as? Map<*, *>&#10;                println("Document project name: ${project?.get("name")}")&#10;            }&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

This should return Document 2 (the task document).

**Example 5: Find meeting notes where Bob is the second attendee (Array index and nested object)**

Arrays are 0-indexed. We access the second element of `attendees` with `.1`, then its `name` field.

```bash {"id":"01J10B0F5H8G9H0J1K2L3M4N5V"}
# Query: content.attendees[1].name equals "Bob The Builder"
curl -G -s "http://localhost:8080/documents" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 --data-urlencode 'content_query=attendees.1.name equals "Bob The Builder"' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
params = {
    # Note the use of .1. to access the second element (index 1) of the array
    "content_query": 'attendees.1.name equals "Bob The Builder"'
}
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.get(url, headers=headers, params=params)
response.raise_for_status()
result = response.json()&#10;
# Print matching documents
print(json.dumps(result, indent=2))&#10;
# Verify we found the meeting notes with Bob as the second attendee
if result.get("data") and len(result["data"]) > 0:
    for doc in result["data"]:
        content = doc.get("content", {})
        attendees = content.get("attendees", [])
        if len(attendees) > 1:
            second_attendee = attendees[1]
            print(f"Second attendee name: {second_attendee.get('name')}")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;// Note the use of .1. to access the second element (index 1) of the array&#10;HttpUrl url = HttpUrl.parse("http://localhost:8080/documents")&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "attendees.1.name equals \"Bob The Builder\"")&#10;    .build();&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print matching documents&#10;  System.out.println(gson.toJson(responseMap));&#10;  &#10;  // Verify we found the meeting notes with Bob as the second attendee&#10;  List<Map<String, Object>> data = (List<Map<String, Object>>) responseMap.get("data");&#10;  if (data != null && !data.isEmpty()) {&#10;    for (Map<String, Object> doc : data) {&#10;      Map<String, Object> content = (Map<String, Object>) doc.get("content");&#10;      List<Map<String, Object>> attendees = (List<Map<String, Object>>) content.get("attendees");&#10;      if (attendees != null && attendees.size() > 1) {&#10;        Map<String, Object> secondAttendee = attendees.get(1);&#10;        System.out.println("Second attendee name: " + secondAttendee.get("name"));&#10;      }&#10;    }&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;// Note the use of .1. to access the second element (index 1) of the array&#10;val url = HttpUrl.parse("http://localhost:8080/documents")!!&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "attendees.1.name equals \"Bob The Builder\"")&#10;    .build()&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print matching documents&#10;        println(gson.toJson(responseMap))&#10;        &#10;        // Verify we found the meeting notes with Bob as the second attendee&#10;        val data = responseMap["data"] as? List<Map<String, Any>>&#10;        if (data != null && data.isNotEmpty()) {&#10;            data.forEach { doc ->&#10;                val content = doc["content"] as? Map<*, *>&#10;                val attendees = content?.get("attendees") as? List<Map<*, *>>&#10;                if (attendees != null && attendees.size > 1) {&#10;                    val secondAttendee = attendees[1]&#10;                    println("Second attendee name: ${secondAttendee["name"]}")&#10;                }&#10;            }&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

This should return Document 3 (the meeting notes).

**Example 6: Find items tagged "urgent" (Array `contains`)**

The `contains` operator can check if an array contains a specific primitive value.

```bash {"id":"01J10B0F5J0G1H2J3K4L5M6N7W"}
# Query: content.tags contains "urgent"
curl -G -s "http://localhost:8080/documents" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 --data-urlencode 'content_query=tags contains "urgent"' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
params = {
    # Use 'contains' operator to check if the 'tags' array contains "urgent"
    "content_query": 'tags contains "urgent"'
}
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.get(url, headers=headers, params=params)
response.raise_for_status()
result = response.json()&#10;
# Print matching documents
print(json.dumps(result, indent=2))&#10;
# Verify we found documents tagged "urgent"
if result.get("data") and len(result["data"]) > 0:
    for doc in result["data"]:
        content = doc.get("content", {})
        tags = content.get("tags", [])
        if "urgent" in tags:
            print(f"Document ID {doc.get('id')} has 'urgent' tag.")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;// Use 'contains' operator to check if the 'tags' array contains "urgent"&#10;HttpUrl url = HttpUrl.parse("http://localhost:8080/documents")&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "tags contains \"urgent\"")&#10;    .build();&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print matching documents&#10;  System.out.println(gson.toJson(responseMap));&#10;  &#10;  // Verify we found documents tagged "urgent"&#10;  List<Map<String, Object>> data = (List<Map<String, Object>>) responseMap.get("data");&#10;  if (data != null && !data.isEmpty()) {&#10;    for (Map<String, Object> doc : data) {&#10;      Map<String, Object> content = (Map<String, Object>) doc.get("content");&#10;      List<String> tags = (List<String>) content.get("tags");&#10;      if (tags != null && tags.contains("urgent")) {&#10;        System.out.println("Document ID " + doc.get("id") + " has 'urgent' tag.");&#10;      }&#10;    }&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;// Use 'contains' operator to check if the 'tags' array contains "urgent"&#10;val url = HttpUrl.parse("http://localhost:8080/documents")!!&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "tags contains \"urgent\"")&#10;    .build()&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print matching documents&#10;        println(gson.toJson(responseMap))&#10;        &#10;        // Verify we found documents tagged "urgent"&#10;        val data = responseMap["data"] as? List<Map<String, Any>>&#10;        if (data != null && data.isNotEmpty()) {&#10;            data.forEach { doc ->&#10;                val content = doc["content"] as? Map<*, *>&#10;                val tags = content?.get("tags") as? List<*>&#10;                if (tags?.contains("urgent") == true) {&#10;                    println("Document ID ${doc["id"]} has 'urgent' tag.")&#10;                }&#10;            }&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

This should return Document 2 (the task document), as its `tags` array includes "urgent".

**Example 7: Find meeting notes with "milestones" in the summary (String `contains`)**

```bash {"id":"01J10B0F5J8G9H0J1K2L3M4N5X"}
# Query: content.summary contains "milestones"
curl -G -s "http://localhost:8080/documents" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 --data-urlencode 'content_query=summary contains "milestones"' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
params = {
    # Use 'contains' operator to check if the 'summary' string contains "milestones"
    "content_query": 'summary contains "milestones"'
}
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.get(url, headers=headers, params=params)
response.raise_for_status()
result = response.json()&#10;
# Print matching documents
print(json.dumps(result, indent=2))&#10;
# Verify we found documents with "milestones" in the summary
if result.get("data") and len(result["data"]) > 0:
    for doc in result["data"]:
        content = doc.get("content", {})
        summary = content.get("summary", "")
        if "milestones" in summary:
            print(f"Document ID {doc.get('id')} summary contains 'milestones'.")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;// Use 'contains' operator to check if the 'summary' string contains "milestones"&#10;HttpUrl url = HttpUrl.parse("http://localhost:8080/documents")&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "summary contains \"milestones\"")&#10;    .build();&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print matching documents&#10;  System.out.println(gson.toJson(responseMap));&#10;  &#10;  // Verify we found documents with "milestones" in the summary&#10;  List<Map<String, Object>> data = (List<Map<String, Object>>) responseMap.get("data");&#10;  if (data != null && !data.isEmpty()) {&#10;    for (Map<String, Object> doc : data) {&#10;      Map<String, Object> content = (Map<String, Object>) doc.get("content");&#10;      String summary = (String) content.get("summary");&#10;      if (summary != null && summary.contains("milestones")) {&#10;        System.out.println("Document ID " + doc.get("id") + " summary contains 'milestones'.");&#10;      }&#10;    }&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;// Build URL with query parameters manually to ensure proper encoding&#10;// Use 'contains' operator to check if the 'summary' string contains "milestones"&#10;val url = HttpUrl.parse("http://localhost:8080/documents")!!&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "summary contains \"milestones\"")&#10;    .build()&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print matching documents&#10;        println(gson.toJson(responseMap))&#10;        &#10;        // Verify we found documents with "milestones" in the summary&#10;        val data = responseMap["data"] as? List<Map<String, Any>>&#10;        if (data != null && data.isNotEmpty()) {&#10;            data.forEach { doc ->&#10;                val content = doc["content"] as? Map<*, *>&#10;                val summary = content?.get("summary") as? String&#10;                if (summary?.contains("milestones") == true) {&#10;                    println("Document ID ${doc["id"]} summary contains 'milestones'.")&#10;                }&#10;            }&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

This should return Document 3 (the meeting notes).

### Step 13: Combining Logic (`or`)

Let's find documents that are either tasks OR are tagged "urgent". We use `content_query=or` (lowercase) between the two conditions.

```bash {"id":"01J10B0F5K0G1H2J3K4L5M6N7Y"}
# Query: content.type equals "task" OR content.tags contains "urgent"
curl -G -s "http://localhost:8080/documents" \
 -H "Authorization: Bearer $ALICE_TOKEN" \
 --data-urlencode 'content_query=type equals "task"' \
 --data-urlencode 'content_query=or' \
 --data-urlencode 'content_query=tags contains "urgent"' | jq .
```

<details><summary>🐍 Python</summary><pre><code>import requests
import json&#10;
# ALICE_TOKEN is already initialized&#10;
url = "http://localhost:8080/documents"
# Combine conditions with 'or'
params = [
    ('content_query', 'type equals "task"'),
    ('content_query', 'or'),
    ('content_query', 'tags contains "urgent"')
]
headers = {
    "Authorization": f"Bearer {ALICE_TOKEN}"
}&#10;
response = requests.get(url, headers=headers, params=params)
response.raise_for_status()
result = response.json()&#10;
# Print matching documents
print(json.dumps(result, indent=2))&#10;
# Verify we found documents that are tasks OR are tagged "urgent"
if result.get("data") and len(result["data"]) > 0:
    for doc in result["data"]:
        content = doc.get("content", {})
        doc_type = content.get("type")
        tags = content.get("tags", [])
        if doc_type == "task" or "urgent" in tags:
            print(f"Document ID {doc.get('id')} matches (Type: {doc_type}, Tags: {tags})")
</code></pre></details>
<details><summary>☕ Java (OkHttp + Gson)</summary><pre><code>import okhttp3.*;&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;import java.util.Map;&#10;import java.util.List;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;OkHttpClient client = new OkHttpClient();&#10;Gson gson = new GsonBuilder().setPrettyPrinting().create();&#10;&#10;// Build URL with query parameters manually, combining with 'or'&#10;HttpUrl url = HttpUrl.parse("http://localhost:8080/documents")&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "type equals \"task\"")&#10;    .addQueryParameter("content_query", "or")&#10;    .addQueryParameter("content_query", "tags contains \"urgent\"")&#10;    .build();&#10;&#10;Request request = new Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer " + ALICE_TOKEN)&#10;    .build();&#10;&#10;try (Response response = client.newCall(request).execute()) {&#10;  if (!response.isSuccessful()) throw new IOException("Unexpected code " + response);&#10;  &#10;  Map responseMap = gson.fromJson(response.body().charStream(), Map.class);&#10;  &#10;  // Print matching documents&#10;  System.out.println(gson.toJson(responseMap));&#10;  &#10;  // Verify we found documents that are tasks OR are tagged "urgent"&#10;  List<Map<String, Object>> data = (List<Map<String, Object>>) responseMap.get("data");&#10;  if (data != null && !data.isEmpty()) {&#10;    for (Map<String, Object> doc : data) {&#10;      Map<String, Object> content = (Map<String, Object>) doc.get("content");&#10;      String docType = (String) content.get("type");&#10;      List<String> tags = (List<String>) content.get("tags");&#10;      boolean isTask = "task".equals(docType);&#10;      boolean hasUrgentTag = tags != null && tags.contains("urgent");&#10;      if (isTask || hasUrgentTag) {&#10;        System.out.println("Document ID " + doc.get("id") + " matches (Type: " + docType + ", Tags: " + tags + ")");&#10;      }&#10;    }&#10;  }&#10;} catch (IOException e) {&#10;  e.printStackTrace();&#10;}</code></pre></details>
<details><summary>📱 Kotlin (OkHttp + Gson)</summary><pre><code>import okhttp3.*&#10;import java.io.IOException;&#10;import com.google.gson.Gson;&#10;import com.google.gson.GsonBuilder;&#10;&#10;// ALICE_TOKEN is already initialized&#10;&#10;val client = OkHttpClient()&#10;val gson = GsonBuilder().setPrettyPrinting().create()&#10;&#10;// Build URL with query parameters manually, combining with 'or'&#10;val url = HttpUrl.parse("http://localhost:8080/documents")!!&#10;    .newBuilder()&#10;    .addQueryParameter("content_query", "type equals \"task\"")&#10;    .addQueryParameter("content_query", "or")&#10;    .addQueryParameter("content_query", "tags contains \"urgent\"")&#10;    .build()&#10;&#10;val request = Request.Builder()&#10;    .url(url)&#10;    .get()&#10;    .addHeader("Authorization", "Bearer $ALICE_TOKEN")&#10;    .build()&#10;&#10;try {&#10;    client.newCall(request).execute().use { response ->&#10;        if (!response.isSuccessful) throw IOException("Unexpected code $response")&#10;        &#10;        val responseMap = gson.fromJson(response.body!!.charStream(), Map::class.java)&#10;        &#10;        // Print matching documents&#10;        println(gson.toJson(responseMap))&#10;        &#10;        // Verify we found documents that are tasks OR are tagged "urgent"&#10;        val data = responseMap["data"] as? List<Map<String, Any>>&#10;        if (data != null && data.isNotEmpty()) {&#10;            data.forEach { doc ->&#10;                val content = doc["content"] as? Map<*, *>&#10;                val docType = content?.get("type") as? String&#10;                val tags = content?.get("tags") as? List<*>&#10;                val isTask = docType == "task"&#10;                val hasUrgentTag = tags?.contains("urgent") == true&#10;                if (isTask || hasUrgentTag) {&#10;                    println("Document ID ${doc["id"]} matches (Type: $docType, Tags: $tags)")&#10;                }&#10;            }&#10;        }&#10;    }&#10;} catch (e: IOException) {&#10;    e.printStackTrace()&#10;}</code></pre></details>&#10;

**Output Explanation:**

This should return only Document 2. Although the query uses `OR`, Document 2 matches both conditions (`type` is "task" AND `tags` contains "urgent"). If we had another document tagged "urgent" but not a task, it would also appear here.

---

This covers the basics of content querying. You can combine these techniques to build complex searches based on the specific structure of your JSON documents. Remember to consult the API documentation (available via Swagger UI when the server runs) for the full list of operators and syntax details.
