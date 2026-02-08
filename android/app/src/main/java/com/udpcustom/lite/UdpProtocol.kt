package com.udpcustom.lite

import java.io.EOFException
import java.io.InputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.security.MessageDigest
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

data class ClientConfig(
    val serverHost: String,
    val anyUdpPort: Boolean,
    val portMin: Int,
    val portMax: Int,
    val token: String,
    val localPort: Int
)

object ConfigParser {
    private const val DEFAULT_LOCAL_PORT = 5300

    fun parse(raw: String): ClientConfig {
        require(raw.isNotBlank()) { "config kosong" }
        val at = raw.lastIndexOf('@')
        require(at > 0) { "format invalid: missing @" }

        val left = raw.substring(0, at)
        val right = raw.substring(at + 1)

        val colonLeft = left.lastIndexOf(':')
        require(colonLeft > 0 && colonLeft < left.length - 1) { "format invalid host:portSpec" }
        val host = left.substring(0, colonLeft)
        val portSpec = left.substring(colonLeft + 1)

        val colonRight = right.lastIndexOf(':')
        val token: String
        val localPort: Int
        if (colonRight <= 0 || colonRight == right.length - 1) {
            token = right
            localPort = DEFAULT_LOCAL_PORT
        } else {
            val maybePort = right.substring(colonRight + 1)
            val parsed = maybePort.toIntOrNull()
            if (parsed != null && parsed in 1..65535) {
                token = right.substring(0, colonRight)
                localPort = parsed
            } else {
                token = right
                localPort = DEFAULT_LOCAL_PORT
            }
        }
        require(token.isNotBlank()) { "token kosong" }

        if (portSpec == "1-65535") {
            return ClientConfig(host, true, 1, 65535, token, localPort)
        }

        if (portSpec.contains("-")) {
            val pieces = portSpec.split("-")
            require(pieces.size == 2) { "range invalid" }
            val min = parsePort(pieces[0])
            val max = parsePort(pieces[1])
            require(min <= max) { "range invalid" }
            return ClientConfig(host, false, min, max, token, localPort)
        }

        val one = parsePort(portSpec)
        return ClientConfig(host, false, one, one, token, localPort)
    }

    private fun parsePort(v: String): Int {
        val n = v.toIntOrNull() ?: error("port harus numeric")
        require(n in 1..65535) { "port harus 1..65535" }
        return n
    }
}

data class FrameHeader(
    val flags: Int,
    val sessionId: Long,
    val dstPort: Int,
    val payloadLen: Int
)

data class Frame(val header: FrameHeader, val payload: ByteArray)

object UdpFraming {
    private val magic = byteArrayOf('U'.code.toByte(), 'D'.code.toByte(), 'P'.code.toByte(), 'C'.code.toByte())
    private const val version: Byte = 1
    const val FLAG_DATA = 0x01
    const val FLAG_KEEPALIVE = 0x02
    const val MAX_PAYLOAD = 1200

    fun encode(frame: Frame, token: String): ByteArray {
        require(frame.payload.size <= MAX_PAYLOAD) { "payload oversize" }
        require(frame.header.dstPort in 1..65535) { "dstPort invalid" }

        val header = ByteBuffer.allocate(18).order(ByteOrder.BIG_ENDIAN)
        header.put(magic)
        header.put(version)
        header.put(frame.header.flags.toByte())
        header.putLong(frame.header.sessionId)
        header.putShort(frame.header.dstPort.toShort())
        header.putShort(frame.payload.size.toShort())

        val body = header.array() + frame.payload
        val hmac = hmacSha256(body, token.toByteArray())
        return body + byteArrayOf(32) + hmac
    }

    fun decode(input: InputStream, token: String): Frame {
        val headerBytes = input.readExact(18)
        require(headerBytes.copyOfRange(0, 4).contentEquals(magic)) { "magic invalid" }
        require(headerBytes[4] == version) { "version invalid" }

        val bb = ByteBuffer.wrap(headerBytes).order(ByteOrder.BIG_ENDIAN)
        bb.position(5)
        val flags = bb.get().toInt() and 0xff
        val sessionId = bb.long
        val dstPort = bb.short.toInt() and 0xffff
        val payloadLen = bb.short.toInt() and 0xffff
        require(payloadLen <= MAX_PAYLOAD) { "payload too large" }

        val payload = input.readExact(payloadLen)
        val hLen = input.readExact(1)[0].toInt() and 0xff
        require(hLen == 32) { "hmac length invalid" }
        val hmac = input.readExact(32)

        val signed = headerBytes + payload
        val expected = hmacSha256(signed, token.toByteArray())
        require(MessageDigest.isEqual(expected, hmac)) { "hmac invalid" }

        return Frame(FrameHeader(flags, sessionId, dstPort, payloadLen), payload)
    }

    private fun hmacSha256(data: ByteArray, key: ByteArray): ByteArray {
        val mac = Mac.getInstance("HmacSHA256")
        mac.init(SecretKeySpec(key, "HmacSHA256"))
        return mac.doFinal(data)
    }

    private fun InputStream.readExact(n: Int): ByteArray {
        val out = ByteArray(n)
        var read = 0
        while (read < n) {
            val r = this.read(out, read, n - read)
            if (r < 0) throw EOFException("stream ended")
            read += r
        }
        return out
    }
}
