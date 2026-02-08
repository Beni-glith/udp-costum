package com.udpcustom.lite

import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.concurrent.thread
import kotlin.random.Random

class UdpTunnelClient(
    private val config: ClientConfig,
    private val dstHost: String,
    private val dstPort: Int,
    private val serverTcpPort: Int,
    private val logger: (String) -> Unit
) {
    private val running = AtomicBoolean(false)
    private var udpSocket: DatagramSocket? = null
    private var tunnelThread: Thread? = null
    private val sessionBySender = ConcurrentHashMap<String, Long>()
    private val senderBySession = ConcurrentHashMap<Long, InetSocketAddress>()

    fun start() {
        require(dstPort in 1..65535) { "dst port invalid" }
        if (!config.anyUdpPort) {
            require(dstPort in config.portMin..config.portMax) { "dst port tidak diizinkan" }
        }

        if (!running.compareAndSet(false, true)) return

        udpSocket = DatagramSocket(config.localPort, InetAddress.getByName("127.0.0.1"))
        logger("UDP listen 127.0.0.1:${config.localPort}, dst=$dstHost:$dstPort")

        tunnelThread = thread(name = "udp-tunnel-loop") {
            while (running.get()) {
                runOnce()
                if (running.get()) {
                    logger("TCP putus, reconnect 2 detik...")
                    Thread.sleep(2000)
                }
            }
        }
    }

    fun stop() {
        running.set(false)
        udpSocket?.close()
        tunnelThread?.interrupt()
        logger("Tunnel berhenti")
    }

    private fun runOnce() {
        val socket = Socket()
        socket.connect(InetSocketAddress(config.serverHost, serverTcpPort), 5000)
        socket.tcpNoDelay = true
        logger("TCP connected ${config.serverHost}:$serverTcpPort")

        val input = socket.getInputStream()
        val output = socket.getOutputStream()
        val udp = udpSocket ?: return

        val recvThread = thread(name = "tcp-recv") {
            try {
                while (running.get()) {
                    val frame = UdpFraming.decode(input, config.token)
                    if ((frame.header.flags and UdpFraming.FLAG_DATA) == 0) continue
                    val sender = senderBySession[frame.header.sessionId] ?: continue
                    val pkt = DatagramPacket(frame.payload, frame.payload.size, sender.address, sender.port)
                    udp.send(pkt)
                }
            } catch (_: Exception) {
            }
        }

        val buf = ByteArray(65535)
        val udpPacket = DatagramPacket(buf, buf.size)
        var lastSend = System.currentTimeMillis()

        try {
            udp.soTimeout = 1000
            while (running.get() && !socket.isClosed) {
                try {
                    udp.receive(udpPacket)
                    if (udpPacket.length > UdpFraming.MAX_PAYLOAD) {
                        logger("drop oversize ${udpPacket.length}")
                        continue
                    }
                    val sender = InetSocketAddress(udpPacket.address, udpPacket.port)
                    val key = "${sender.address.hostAddress}:${sender.port}"
                    val sid = sessionBySender.computeIfAbsent(key) { Random.nextLong() }
                    senderBySession[sid] = sender

                    val payload = udpPacket.data.copyOfRange(udpPacket.offset, udpPacket.offset + udpPacket.length)
                    val frame = Frame(FrameHeader(UdpFraming.FLAG_DATA, sid, dstPort, payload.size), payload)
                    val enc = UdpFraming.encode(frame, config.token)
                    output.write(enc)
                    output.flush()
                    lastSend = System.currentTimeMillis()
                } catch (_: Exception) {
                    val idle = System.currentTimeMillis() - lastSend
                    if (idle >= 15_000) {
                        val ka = Frame(FrameHeader(UdpFraming.FLAG_KEEPALIVE, 0L, 1, 0), byteArrayOf())
                        output.write(UdpFraming.encode(ka, config.token))
                        output.flush()
                        lastSend = System.currentTimeMillis()
                    }
                }
            }
        } finally {
            socket.close()
            recvThread.interrupt()
        }
    }
}
